package thumbnail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	StatusReady       = "ready"
	StatusUnavailable = "unavailable"
	StatusUnsupported = "unsupported"

	defaultMaxPageBytes  = 512 * 1024
	defaultMaxImageBytes = 2 * 1024 * 1024
)

var supportedSources = map[string]bool{
	"thingiverse": true,
	"printables":  true,
	"makerworld":  true,
}

type Fetcher struct {
	Client            *http.Client
	ThumbnailDir      string
	MaxPageBytes      int64
	MaxImageBytes     int64
	AllowPrivateHosts bool
}

type Result struct {
	Title       string
	ImageURL    string
	ImageSource string
	Path        string
	ContentType string
	Status      string
	Error       string
	CheckedAt   time.Time
}

type Metadata struct {
	Title       string
	ImageURL    string
	ImageSource string
}

func SupportedSource(source string) bool {
	return supportedSources[strings.ToLower(strings.TrimSpace(source))]
}

func (f Fetcher) Fetch(ctx context.Context, linkID int64, linkURL, sourceType string) Result {
	result := Result{Status: StatusUnavailable, CheckedAt: time.Now().UTC()}
	if !SupportedSource(sourceType) {
		result.Status = StatusUnsupported
		result.Error = "thumbnail discovery is only enabled for Thingiverse, Printables, and MakerWorld links"
		return result
	}
	if strings.TrimSpace(f.ThumbnailDir) == "" {
		result.Error = "thumbnail cache directory is not configured"
		return result
	}
	pageURL, err := parseHTTPURL(linkURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if err := f.validateURL(ctx, pageURL); err != nil {
		result.Error = err.Error()
		return result
	}

	client := f.client(ctx)
	pageResp, err := client.Get(pageURL.String())
	if err != nil {
		result.Error = fmt.Sprintf("fetch page: %v", err)
		return result
	}
	defer pageResp.Body.Close()
	if err := f.validateURL(ctx, pageResp.Request.URL); err != nil {
		result.Error = err.Error()
		return result
	}
	if pageResp.StatusCode < 200 || pageResp.StatusCode > 299 {
		result.Error = fmt.Sprintf("fetch page: unexpected status %d", pageResp.StatusCode)
		return result
	}
	pageBody, err := readLimited(pageResp.Body, f.maxPageBytes())
	if err != nil {
		result.Error = fmt.Sprintf("read page: %v", err)
		return result
	}
	metadata := ExtractMetadata(pageBody, pageResp.Request.URL)
	result.Title = metadata.Title
	result.ImageURL = metadata.ImageURL
	result.ImageSource = metadata.ImageSource
	if metadata.ImageURL == "" {
		result.Error = "no supported Open Graph or Twitter image metadata found"
		return result
	}

	imageURL, err := parseHTTPURL(metadata.ImageURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if err := f.validateURL(ctx, imageURL); err != nil {
		result.Error = err.Error()
		return result
	}
	imageResp, err := client.Get(imageURL.String())
	if err != nil {
		result.Error = fmt.Sprintf("fetch image: %v", err)
		return result
	}
	defer imageResp.Body.Close()
	if err := f.validateURL(ctx, imageResp.Request.URL); err != nil {
		result.Error = err.Error()
		return result
	}
	if imageResp.StatusCode < 200 || imageResp.StatusCode > 299 {
		result.Error = fmt.Sprintf("fetch image: unexpected status %d", imageResp.StatusCode)
		return result
	}

	headerType := cleanContentType(imageResp.Header.Get("Content-Type"))
	if headerType != "" && !allowedImageType(headerType) {
		result.Error = fmt.Sprintf("image content type %q is not allowed", headerType)
		return result
	}
	imageBody, err := readLimited(imageResp.Body, f.maxImageBytes())
	if err != nil {
		result.Error = fmt.Sprintf("read image: %v", err)
		return result
	}
	contentType := headerType
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(imageBody)
	}
	contentType = cleanContentType(contentType)
	if !allowedImageType(contentType) {
		result.Error = fmt.Sprintf("image content type %q is not allowed", contentType)
		return result
	}

	if err := os.MkdirAll(f.ThumbnailDir, 0o755); err != nil {
		result.Error = fmt.Sprintf("prepare thumbnail cache: %v", err)
		return result
	}
	name := cacheFilename(linkID, imageURL.String(), contentType)
	targetPath := filepath.Join(f.ThumbnailDir, name)
	if err := os.WriteFile(targetPath, imageBody, 0o644); err != nil {
		result.Error = fmt.Sprintf("write thumbnail cache: %v", err)
		return result
	}

	result.Status = StatusReady
	result.Path = filepath.Join(filepath.Base(f.ThumbnailDir), name)
	result.ContentType = contentType
	result.Error = ""
	return result
}

func ExtractMetadata(body []byte, base *url.URL) Metadata {
	attrsByProperty := make(map[string]map[string]string)
	for _, match := range metaTagPattern.FindAllSubmatch(body, -1) {
		attrs := parseAttrs(string(match[1]))
		key := strings.ToLower(firstNonEmpty(attrs["property"], attrs["name"]))
		if key != "" {
			attrsByProperty[key] = attrs
		}
	}

	title := metaContent(attrsByProperty, "og:title")
	if title == "" {
		title = metaContent(attrsByProperty, "twitter:title")
	}
	imageKeys := []string{"og:image:secure_url", "og:image", "twitter:image", "twitter:image:src"}
	for _, key := range imageKeys {
		if image := resolveMetadataURL(metaContent(attrsByProperty, key), base); image != "" {
			return Metadata{Title: title, ImageURL: image, ImageSource: key}
		}
	}
	return Metadata{Title: title}
}

func ValidatePublicHTTPURL(ctx context.Context, rawURL string) error {
	parsed, err := parseHTTPURL(rawURL)
	if err != nil {
		return err
	}
	return validatePublicHost(ctx, parsed.Hostname())
}

func (f Fetcher) validateURL(ctx context.Context, parsed *url.URL) error {
	if f.AllowPrivateHosts {
		return nil
	}
	return validatePublicHost(ctx, parsed.Hostname())
}

func validatePublicHost(ctx context.Context, hostname string) error {
	host := strings.TrimSpace(hostname)
	if host == "" {
		return errors.New("url host is required")
	}
	if strings.EqualFold(host, "localhost") {
		return errors.New("localhost urls are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("url host %q resolves to a private or local address", host)
		}
		return nil
	}
	resolver := net.DefaultResolver
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("resolve host %q: no addresses found", host)
	}
	for _, addr := range ips {
		if !isPublicIP(addr.IP) {
			return fmt.Errorf("url host %q resolves to a private or local address", host)
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}

func parseHTTPURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("url is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("url must use http or https")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("url host is required")
	}
	return parsed, nil
}

func (f Fetcher) client(ctx context.Context) *http.Client {
	var base *http.Client
	if f.Client != nil {
		base = f.Client
	} else {
		base = &http.Client{Timeout: 6 * time.Second}
	}
	client := *base
	if client.Timeout == 0 {
		client.Timeout = 6 * time.Second
	}
	previousCheck := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if previousCheck != nil {
			if err := previousCheck(req, via); err != nil {
				return err
			}
		} else if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return f.validateURL(ctx, req.URL)
	}
	return &client
}

func (f Fetcher) maxPageBytes() int64 {
	if f.MaxPageBytes > 0 {
		return f.MaxPageBytes
	}
	return defaultMaxPageBytes
}

func (f Fetcher) maxImageBytes() int64 {
	if f.MaxImageBytes > 0 {
		return f.MaxImageBytes
	}
	return defaultMaxImageBytes
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	var out bytes.Buffer
	written, err := io.Copy(&out, io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if written > limit {
		return nil, fmt.Errorf("response is larger than %d bytes", limit)
	}
	if written == 0 {
		return nil, errors.New("response is empty")
	}
	return out.Bytes(), nil
}

func allowedImageType(contentType string) bool {
	switch cleanContentType(contentType) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func cleanContentType(value string) string {
	contentType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(contentType)
}

func cacheFilename(linkID int64, imageURL, contentType string) string {
	sum := sha256.Sum256([]byte(imageURL))
	return fmt.Sprintf("%d-%s%s", linkID, hex.EncodeToString(sum[:8]), extensionForType(contentType))
}

func extensionForType(contentType string) string {
	switch cleanContentType(contentType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".img"
	}
}

var (
	metaTagPattern = regexp.MustCompile(`(?is)<meta\s+([^>]+)>`)
	attrPattern    = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'>]+))`)
)

func parseAttrs(raw string) map[string]string {
	attrs := make(map[string]string)
	for _, match := range attrPattern.FindAllStringSubmatch(raw, -1) {
		value := firstNonEmpty(match[3], match[4], match[5])
		attrs[strings.ToLower(match[1])] = html.UnescapeString(strings.TrimSpace(value))
	}
	return attrs
}

func metaContent(attrs map[string]map[string]string, key string) string {
	if attrs[key] == nil {
		return ""
	}
	return strings.TrimSpace(attrs[key]["content"])
}

func resolveMetadataURL(raw string, base *url.URL) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if !parsed.IsAbs() && base != nil {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
