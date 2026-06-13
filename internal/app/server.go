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

	"github.com/zachatrocity/3do/internal/auth"
	"github.com/zachatrocity/3do/internal/config"
	"github.com/zachatrocity/3do/internal/store"
)

const (
	sessionCookieName = "3do_session"
	sessionTTL        = 14 * 24 * time.Hour
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
	s.mux.HandleFunc("GET /api/bootstrap", s.bootstrapStatus)
	s.mux.HandleFunc("POST /api/bootstrap/admin", s.bootstrapAdmin)
	s.mux.HandleFunc("GET /api/session", s.session)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.logout)
	s.mux.HandleFunc("GET /api/users", s.requireAuth(s.listUsers, true))
	s.mux.HandleFunc("POST /api/users", s.requireAuth(s.createUser, true))
	s.mux.HandleFunc("PATCH /api/users/{id}", s.requireAuth(s.updateUser, true))
	s.mux.HandleFunc("DELETE /api/users/{id}", s.requireAuth(s.deleteUser, true))
	s.mux.HandleFunc("GET /api/queue-items", s.requireAuth(s.listQueueItems, false))
	s.mux.HandleFunc("POST /api/queue-items", s.requireAuth(s.createQueueItem, false))
	s.mux.HandleFunc("GET /api/queue-items/{id}", s.requireAuth(s.getQueueItem, false))
	s.mux.HandleFunc("PATCH /api/queue-items/{id}", s.requireAuth(s.updateQueueItem, false))
	s.mux.HandleFunc("POST /api/queue-items/{id}/notes", s.requireAuth(s.addQueueItemNote, false))
	s.mux.HandleFunc("GET /api/printers", s.requireAuth(s.listPrinters, true))
	s.mux.HandleFunc("POST /api/printers", s.requireAuth(s.createPrinter, true))
	s.mux.Handle("/", http.FileServer(http.Dir("web")))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) bootstrapStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"required": count == 0})
}

func (s *Server) bootstrapAdmin(w http.ResponseWriter, r *http.Request) {
	var input userPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.BootstrapAdmin(r.Context(), store.UserInput{
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	user, ok := s.userFromRequest(r)
	if !ok {
		count, err := s.store.UserCount(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":              "authentication required",
			"bootstrap_required": count == 0,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]store.User{"user": user})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input loginPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.GetUserByEmail(r.Context(), input.Email)
	if err != nil || !user.Active || !auth.CheckPassword(user.PasswordHash, input.Password) {
		writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
		return
	}
	if err := s.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) requireAuth(next http.HandlerFunc, adminOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.userFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		if adminOnly && user.Role != store.RoleAdmin {
			writeError(w, http.StatusForbidden, errors.New("admin role required"))
			return
		}
		next(w, r)
	}
}

func (s *Server) userFromRequest(r *http.Request) (store.User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return store.User{}, false
	}
	user, err := s.store.SessionUser(r.Context(), auth.SessionTokenHash(s.cfg.SessionSecret, cookie.Value), time.Now())
	if err != nil {
		return store.User{}, false
	}
	return user, true
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user store.User) error {
	token, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(sessionTTL)
	if err := s.store.CreateSession(r.Context(), auth.SessionTokenHash(s.cfg.SessionSecret, token), user.ID, expiresAt); err != nil {
		return err
	}
	_ = s.store.PruneExpiredSessions(r.Context(), time.Now())
	http.SetCookie(w, s.sessionCookie(token, expiresAt, int(sessionTTL.Seconds())))
	return nil
}

func (s *Server) sessionCookie(value string, expiresAt time.Time, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(strings.ToLower(s.cfg.AppURL), "https://"),
	}
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.store.DeleteSession(r.Context(), auth.SessionTokenHash(s.cfg.SessionSecret, cookie.Value))
	}
	http.SetCookie(w, s.sessionCookie("", time.Unix(0, 0), -1))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var input userPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), store.UserInput{
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		Role:         store.UserRole(input.Role),
		Active:       input.activeValue(true),
		PasswordHash: passwordHash,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	var input userPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var passwordHash string
	if input.Password != "" {
		passwordHash, err = auth.HashPassword(input.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	user, err := s.store.UpdateUser(r.Context(), id, store.UserUpdate{
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		Role:         store.UserRole(input.Role),
		Active:       input.activeValue(true),
		PasswordHash: passwordHash,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
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

func (s *Server) getQueueItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"), "queue item")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.GetQueueItem(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
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
		Estimate:    r.FormValue("estimated_minutes"),
		DueAt:       r.FormValue("due_at"),
		Links:       splitLines(r.FormValue("links")),
	}
	item, err := s.store.CreateQueueItem(r.Context(), input.toStoreInput())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, link := range input.Links {
		if _, err := s.store.AddLink(r.Context(), item.ID, link); err != nil {
			s.cleanupQueueItem(r, item.ID)
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	form := r.MultipartForm
	if form != nil {
		for _, header := range form.File["files"] {
			if err := s.persistUpload(r, item.ID, header); err != nil {
				s.cleanupQueueItem(r, item.ID)
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

func (s *Server) cleanupQueueItem(r *http.Request, itemID int64) {
	_ = s.store.DeleteQueueItem(r.Context(), itemID)
	_ = os.RemoveAll(filepath.Join(s.cfg.UploadDir, strconv.FormatInt(itemID, 10)))
}

func (s *Server) updateQueueItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"), "queue item")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input queueItemPayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, _ := s.userFromRequest(r)
	update, err := input.toStoreUpdate(user.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.UpdateQueueItem(r.Context(), id, update)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) addQueueItemNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"), "queue item")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input notePayload
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, _ := s.userFromRequest(r)
	note, err := s.store.AddNote(r.Context(), id, user.DisplayName, input.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) persistUpload(r *http.Request, itemID int64, header *multipart.FileHeader) error {
	if strings.TrimSpace(header.Filename) == "" {
		return errors.New("upload filename is required")
	}
	if !store.AllowedUpload(header.Filename) {
		return fmt.Errorf("unsupported upload type for %q; allowed: .stl, .3mf, .gcode, .step, .stp, .obj, .zip, .png, .jpg, .jpeg, .webp", header.Filename)
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
		_ = os.Remove(targetPath)
		return err
	}
	if written == 0 {
		_ = os.Remove(targetPath)
		return fmt.Errorf("upload %q is empty", header.Filename)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if existing, err := s.store.FileByChecksum(r.Context(), checksum); err == nil {
		_ = os.Remove(targetPath)
		return fmt.Errorf("duplicate upload %q matches existing file %q", header.Filename, existing.OriginalName)
	} else if !errors.Is(err, store.ErrNotFound) {
		_ = os.Remove(targetPath)
		return err
	}
	relativePath, err := filepath.Rel(s.cfg.DataDir, targetPath)
	if err != nil {
		_ = os.Remove(targetPath)
		return err
	}
	_, err = s.store.AddFile(r.Context(), store.ItemFile{
		QueueItemID:  itemID,
		StoragePath:  relativePath,
		OriginalName: header.Filename,
		SizeBytes:    written,
		Checksum:     checksum,
		ContentType:  header.Header.Get("Content-Type"),
		Kind:         store.DetectFileKind(header.Filename),
	})
	if err != nil {
		_ = os.Remove(targetPath)
	}
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
	Estimate    string   `json:"estimated_minutes"`
	DueAt       string   `json:"due_at"`
	StatusNote  string   `json:"status_note"`
	Links       []string `json:"links"`
}

func (p queueItemPayload) toStoreInput() store.QueueItemInput {
	estimate := optionalInt(p.Estimate)
	dueAt, _ := parseOptionalTime(p.DueAt)
	return store.QueueItemInput{
		Title:            p.Title,
		Description:      p.Description,
		Status:           store.QueueStatus(p.Status),
		Priority:         store.Priority(p.Priority),
		RequestedBy:      p.RequestedBy,
		Owner:            p.Owner,
		PrintingBy:       p.PrintingBy,
		Quantity:         p.Quantity,
		Material:         p.Material,
		Color:            p.Color,
		EstimatedMinutes: estimate,
		DueAt:            dueAt,
	}
}

func (p queueItemPayload) toStoreUpdate(actor string) (store.QueueItemUpdate, error) {
	dueAt, err := parseOptionalTime(p.DueAt)
	if err != nil {
		return store.QueueItemUpdate{}, err
	}
	return store.QueueItemUpdate{
		Status:           store.QueueStatus(p.Status),
		Priority:         store.Priority(p.Priority),
		Owner:            p.Owner,
		PrintingBy:       p.PrintingBy,
		Quantity:         p.Quantity,
		Material:         p.Material,
		Color:            p.Color,
		EstimatedMinutes: optionalInt(p.Estimate),
		DueAt:            dueAt,
		Actor:            actor,
		Note:             p.StatusNote,
	}, nil
}

type notePayload struct {
	Body string `json:"body"`
}

type printerPayload struct {
	Name         string `json:"name"`
	Location     string `json:"location"`
	Status       string `json:"status"`
	Capabilities string `json:"capabilities"`
	Notes        string `json:"notes"`
}

type loginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userPayload struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Active      *bool  `json:"active"`
	Password    string `json:"password"`
}

func (p userPayload) activeValue(fallback bool) bool {
	if p.Active == nil {
		return fallback
	}
	return *p.Active
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

func parseID(value, name string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid %s id", name)
	}
	return id, nil
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

func optionalInt(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return nil
	}
	return &parsed
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("due_at must be YYYY-MM-DD or RFC3339")
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
