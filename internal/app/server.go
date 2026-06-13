package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zachatrocity/3do/internal/config"
	"github.com/zachatrocity/3do/internal/store"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	mux   *http.ServeMux
}

func NewServer(cfg config.Config, db *store.Store) http.Handler {
	server := &Server{cfg: cfg, store: db, mux: http.NewServeMux()}
	server.routes()
	return server.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /api/queue-items", s.listQueueItems)
	s.mux.HandleFunc("POST /api/queue-items", s.createQueueItem)
	s.mux.HandleFunc("GET /api/printers", s.listPrinters)
	s.mux.HandleFunc("POST /api/printers", s.createPrinter)
	s.mux.Handle("/", http.FileServer(http.Dir("web")))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listQueueItems(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListQueueItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createQueueItem(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		s.createQueueItemMultipart(w, r)
		return
	}

	var input queueItemPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.CreateQueueItem(r.Context(), input.toStoreInput())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, link := range input.Links {
		if _, err := s.store.AddLink(r.Context(), item.ID, link); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	items, _ := s.store.ListQueueItems(r.Context())
	for _, current := range items {
		if current.ID == item.ID {
			writeJSON(w, http.StatusCreated, current)
			return
		}
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) createQueueItemMultipart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.UploadMaxSize)
	if err := r.ParseMultipartForm(s.cfg.UploadMaxSize); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	input := queueItemPayload{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Status:      r.FormValue("status"),
		Priority:    r.FormValue("priority"),
		RequestedBy: r.FormValue("requested_by"),
		Owner:       r.FormValue("owner"),
		PrintingBy:  r.FormValue("printing_by"),
		Quantity:    parseInt(r.FormValue("quantity")),
		Material:    r.FormValue("material"),
		Color:       r.FormValue("color"),
		Links:       splitLines(r.FormValue("links")),
	}
	item, err := s.store.CreateQueueItem(r.Context(), input.toStoreInput())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, link := range input.Links {
		if _, err := s.store.AddLink(r.Context(), item.ID, link); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	form := r.MultipartForm
	if form != nil {
		for _, header := range form.File["files"] {
			if err := s.persistUpload(r, item.ID, header); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
	}

	items, _ := s.store.ListQueueItems(r.Context())
	for _, current := range items {
		if current.ID == item.ID {
			writeJSON(w, http.StatusCreated, current)
			return
		}
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) persistUpload(r *http.Request, itemID int64, header *multipart.FileHeader) error {
	if !store.AllowedUpload(header.Filename) {
		return fmt.Errorf("unsupported upload type: %s", header.Filename)
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	itemDir := filepath.Join(s.cfg.UploadDir, strconv.FormatInt(itemID, 10))
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return err
	}
	safeName := safeFilename(header.Filename)
	targetPath := filepath.Join(itemDir, fmt.Sprintf("%d-%s", time.Now().UnixNano(), safeName))
	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hasher), file)
	if err != nil {
		return err
	}
	relativePath, err := filepath.Rel(s.cfg.DataDir, targetPath)
	if err != nil {
		return err
	}
	_, err = s.store.AddFile(r.Context(), store.ItemFile{
		QueueItemID:  itemID,
		StoragePath:  relativePath,
		OriginalName: header.Filename,
		SizeBytes:    written,
		Checksum:     hex.EncodeToString(hasher.Sum(nil)),
		ContentType:  header.Header.Get("Content-Type"),
		Kind:         store.DetectFileKind(header.Filename),
	})
	return err
}

func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request) {
	printers, err := s.store.ListPrinters(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, printers)
}

func (s *Server) createPrinter(w http.ResponseWriter, r *http.Request) {
	var input printerPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	printer, err := s.store.CreatePrinter(r.Context(), store.PrinterInput{
		Name:         input.Name,
		Location:     input.Location,
		Status:       input.Status,
		Capabilities: input.Capabilities,
		Notes:        input.Notes,
		Active:       true,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, printer)
}

type queueItemPayload struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	RequestedBy string   `json:"requested_by"`
	Owner       string   `json:"owner"`
	PrintingBy  string   `json:"printing_by"`
	Quantity    int      `json:"quantity"`
	Material    string   `json:"material"`
	Color       string   `json:"color"`
	Links       []string `json:"links"`
}

func (p queueItemPayload) toStoreInput() store.QueueItemInput {
	return store.QueueItemInput{
		Title:       p.Title,
		Description: p.Description,
		Status:      store.QueueStatus(p.Status),
		Priority:    store.Priority(p.Priority),
		RequestedBy: p.RequestedBy,
		Owner:       p.Owner,
		PrintingBy:  p.PrintingBy,
		Quantity:    p.Quantity,
		Material:    p.Material,
		Color:       p.Color,
	}
}

type printerPayload struct {
	Name         string `json:"name"`
	Location     string `json:"location"`
	Status       string `json:"status"`
	Capabilities string `json:"capabilities"`
	Notes        string `json:"notes"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func splitLines(value string) []string {
	var result []string
	for _, line := range strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == ',' }) {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func safeFilename(value string) string {
	value = filepath.Base(value)
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
	if value == "" || value == "." {
		return "upload"
	}
	return value
}
