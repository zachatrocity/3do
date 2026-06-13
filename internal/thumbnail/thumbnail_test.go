package thumbnail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMetadataPrefersOpenGraphImage(t *testing.T) {
	base, _ := url.Parse("https://www.printables.com/model/123-widget")
	body := []byte(`
		<html><head>
			<meta name="twitter:image" content="/twitter.jpg">
			<meta property="og:title" content="Panel Clip &amp; Bracket">
			<meta property="og:image" content="/images/preview.webp">
		</head></html>
	`)

	got := ExtractMetadata(body, base)
	if got.Title != "Panel Clip & Bracket" {
		t.Fatalf("expected title to be unescaped, got %q", got.Title)
	}
	if got.ImageURL != "https://www.printables.com/images/preview.webp" {
		t.Fatalf("expected resolved og image, got %q", got.ImageURL)
	}
	if got.ImageSource != "og:image" {
		t.Fatalf("expected og:image source, got %q", got.ImageSource)
	}
}

func TestValidatePublicHTTPURLRejectsPrivateHosts(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1/model",
		"http://localhost/model",
		"http://10.0.0.4/model",
		"file:///tmp/model",
	} {
		if err := ValidatePublicHTTPURL(context.Background(), rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}

func TestFetcherCachesSupportedImage(t *testing.T) {
	imageBody := []byte("\x89PNG\r\n\x1a\npreview")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/model":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<meta property="og:title" content="Benchy"><meta property="og:image" content="/preview.png">`))
		case "/preview.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	result := Fetcher{
		Client:            server.Client(),
		ThumbnailDir:      dir,
		AllowPrivateHosts: true,
	}.Fetch(context.Background(), 42, server.URL+"/model", "printables")

	if result.Status != StatusReady {
		t.Fatalf("expected ready thumbnail, got %+v", result)
	}
	if result.Title != "Benchy" || result.ImageSource != "og:image" {
		t.Fatalf("unexpected metadata: %+v", result)
	}
	if result.ContentType != "image/png" {
		t.Fatalf("expected image/png, got %q", result.ContentType)
	}
	cachedPath := filepath.Join(filepath.Dir(dir), result.Path)
	if _, err := os.Stat(cachedPath); err != nil {
		t.Fatalf("expected cached file at %s: %v", cachedPath, err)
	}
}

func TestFetcherRejectsOversizedImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/model":
			_, _ = w.Write([]byte(`<meta property="og:image" content="/preview.png">`))
		case "/preview.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 32)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result := Fetcher{
		Client:            server.Client(),
		ThumbnailDir:      t.TempDir(),
		MaxImageBytes:     8,
		AllowPrivateHosts: true,
	}.Fetch(context.Background(), 1, server.URL+"/model", "thingiverse")

	if result.Status != StatusUnavailable {
		t.Fatalf("expected unavailable thumbnail, got %+v", result)
	}
	if !strings.Contains(result.Error, "larger than 8 bytes") {
		t.Fatalf("expected size-limit error, got %q", result.Error)
	}
}

func TestFetcherRejectsUnsupportedSource(t *testing.T) {
	result := Fetcher{ThumbnailDir: t.TempDir()}.Fetch(context.Background(), 1, "https://github.com/example/model", "github")
	if result.Status != StatusUnsupported {
		t.Fatalf("expected unsupported status, got %+v", result)
	}
}
