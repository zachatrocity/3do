package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachatrocity/3do/internal/config"
	"github.com/zachatrocity/3do/internal/store"
)

func TestBootstrapSessionAndLogoutFlow(t *testing.T) {
	handler := newTestServer(t)

	resp := requestJSON(t, handler, http.MethodGet, "/api/queue-items", nil, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected protected API to require auth, got %d: %s", resp.Code, resp.Body.String())
	}

	admin := map[string]any{
		"display_name": "Admin",
		"email":        "admin@example.com",
		"password":     "correct horse battery staple",
	}
	resp = requestJSON(t, handler, http.MethodPost, "/api/bootstrap/admin", admin, nil)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected bootstrap to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	cookie := sessionCookieFrom(t, resp)

	resp = requestJSON(t, handler, http.MethodPost, "/api/bootstrap/admin", admin, nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected second bootstrap to fail, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodGet, "/api/queue-items", nil, cookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected session cookie to authorize API, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodPost, "/api/logout", nil, cookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected logout to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodGet, "/api/queue-items", nil, cookie)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected logged-out cookie to be rejected, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestLoginRejectsInactiveUser(t *testing.T) {
	handler := newTestServer(t)
	adminCookie := bootstrapAdmin(t, handler)

	active := false
	user := map[string]any{
		"display_name": "Member",
		"email":        "member@example.com",
		"password":     "correct horse battery staple",
		"role":         "member",
		"active":       active,
	}
	resp := requestJSON(t, handler, http.MethodPost, "/api/users", user, adminCookie)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected inactive user create to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	login := map[string]any{
		"email":    "member@example.com",
		"password": "correct horse battery staple",
	}
	resp = requestJSON(t, handler, http.MethodPost, "/api/login", login, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected inactive user login to fail, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestMemberCannotManageUsers(t *testing.T) {
	handler := newTestServer(t)
	adminCookie := bootstrapAdmin(t, handler)

	memberCookie := createAndLoginMember(t, handler, adminCookie)

	resp := requestJSON(t, handler, http.MethodGet, "/api/users", nil, memberCookie)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected member user listing to be forbidden, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestPrinterManagementRequiresAdmin(t *testing.T) {
	handler := newTestServer(t)
	adminCookie := bootstrapAdmin(t, handler)
	memberCookie := createAndLoginMember(t, handler, adminCookie)

	printer := map[string]any{
		"name":     "Voron",
		"location": "Bench",
	}
	resp := requestJSON(t, handler, http.MethodPost, "/api/printers", printer, memberCookie)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected member printer create to be forbidden, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodGet, "/api/printers", nil, memberCookie)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected member printer listing to be forbidden, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodPost, "/api/printers", printer, adminCookie)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected admin printer create to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodGet, "/api/printers", nil, adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected admin printer listing to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestQueueItemDetailUpdateAndNotesAPI(t *testing.T) {
	handler := newTestServer(t)
	cookie := bootstrapAdmin(t, handler)

	create := map[string]any{
		"title":        "Bracket",
		"status":       "queued",
		"priority":     "normal",
		"requested_by": "Zach",
		"owner":        "Shop",
		"quantity":     1,
		"material":     "PLA",
		"color":        "Black",
	}
	resp := requestJSON(t, handler, http.MethodPost, "/api/queue-items", create, cookie)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected queue item create to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	var created store.QueueItem
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	update := map[string]any{
		"status":            "printing",
		"priority":          "high",
		"owner":             "Alex",
		"printing_by":       "Voron",
		"quantity":          2,
		"material":          "PETG",
		"color":             "Orange",
		"estimated_minutes": "120",
		"due_at":            "2026-07-04",
		"status_note":       "Loaded plate",
	}
	resp = requestJSON(t, handler, http.MethodPatch, fmt.Sprintf("/api/queue-items/%d", created.ID), update, cookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected queue item update to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	var updated store.QueueItemDetail
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != store.StatusPrinting || updated.Priority != store.PriorityHigh || updated.PrintingBy != "Voron" {
		t.Fatalf("unexpected updated item: %+v", updated.QueueItem)
	}
	if updated.EstimatedMinutes == nil || *updated.EstimatedMinutes != 120 {
		t.Fatalf("expected estimate to round trip, got %+v", updated.EstimatedMinutes)
	}
	if len(updated.StatusEvents) != 2 {
		t.Fatalf("expected create and update events, got %d", len(updated.StatusEvents))
	}

	resp = requestJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/queue-items/%d/notes", created.ID), map[string]any{
		"body": "Supports removed cleanly.",
	}, cookie)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected note create to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodGet, fmt.Sprintf("/api/queue-items/%d", created.ID), nil, cookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected detail fetch to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	var detail store.QueueItemDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Notes) != 1 || detail.Notes[0].Body != "Supports removed cleanly." {
		t.Fatalf("expected note in detail response, got %+v", detail.Notes)
	}
}

func TestCannotDeleteOrDisableLastAdmin(t *testing.T) {
	handler := newTestServer(t)
	adminCookie := bootstrapAdmin(t, handler)

	resp := requestJSON(t, handler, http.MethodGet, "/api/users", nil, adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected user listing to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	var users []store.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("expected one admin, got %d", len(users))
	}

	update := map[string]any{
		"display_name": users[0].DisplayName,
		"email":        users[0].Email,
		"role":         "admin",
		"active":       false,
	}
	resp = requestJSON(t, handler, http.MethodPatch, fmt.Sprintf("/api/users/%d", users[0].ID), update, adminCookie)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected disabling last admin to fail, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodDelete, fmt.Sprintf("/api/users/%d", users[0].ID), nil, adminCookie)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected deleting last admin to fail, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestMultipartUploadRejectsDuplicateAndCleansUpItem(t *testing.T) {
	handler := newTestServer(t)
	cookie := bootstrapAdmin(t, handler)

	resp := requestMultipart(t, handler, map[string]string{
		"title": "Duplicate bracket",
	}, []testUpload{
		{name: "bracket-a.stl", body: "solid duplicate"},
		{name: "bracket-b.stl", body: "solid duplicate"},
	}, cookie)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate upload to fail, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "duplicate upload") {
		t.Fatalf("expected duplicate error, got %s", resp.Body.String())
	}

	resp = requestJSON(t, handler, http.MethodGet, "/api/queue-items", nil, cookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected list to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	var items []store.QueueItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected failed multipart request to clean up queue item, got %d items", len(items))
	}
}

func TestMultipartUploadValidationMessages(t *testing.T) {
	handler := newTestServer(t)
	cookie := bootstrapAdmin(t, handler)

	resp := requestMultipart(t, handler, map[string]string{
		"title": "Bad upload",
	}, []testUpload{{name: "payload.exe", body: "binary"}}, cookie)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported upload to fail, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "allowed: .stl") {
		t.Fatalf("expected allowlist in error, got %s", resp.Body.String())
	}

	resp = requestMultipart(t, handler, map[string]string{
		"title": "Empty upload",
	}, []testUpload{{name: "empty.stl", body: ""}}, cookie)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected empty upload to fail, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "is empty") {
		t.Fatalf("expected empty-file error, got %s", resp.Body.String())
	}
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	return NewServer(config.Config{
		AppURL:        "http://example.test",
		DataDir:       dataDir,
		UploadDir:     filepath.Join(dataDir, "uploads"),
		ThumbnailDir:  filepath.Join(dataDir, "thumbnails"),
		UploadMaxSize: 1024 * 1024,
		SessionSecret: "0123456789abcdef0123456789abcdef",
	}, db)
}

func bootstrapAdmin(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	resp := requestJSON(t, handler, http.MethodPost, "/api/bootstrap/admin", map[string]any{
		"display_name": "Admin",
		"email":        "admin@example.com",
		"password":     "correct horse battery staple",
	}, nil)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected bootstrap to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	return sessionCookieFrom(t, resp)
}

func createAndLoginMember(t *testing.T, handler http.Handler, adminCookie *http.Cookie) *http.Cookie {
	t.Helper()
	user := map[string]any{
		"display_name": "Member",
		"email":        "member@example.com",
		"password":     "correct horse battery staple",
		"role":         "member",
	}
	resp := requestJSON(t, handler, http.MethodPost, "/api/users", user, adminCookie)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected member create to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	login := map[string]any{
		"email":    "member@example.com",
		"password": "correct horse battery staple",
	}
	resp = requestJSON(t, handler, http.MethodPost, "/api/login", login, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected member login to succeed, got %d: %s", resp.Code, resp.Body.String())
	}
	return sessionCookieFrom(t, resp)
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, payload any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

type testUpload struct {
	name string
	body string
}

func requestMultipart(t *testing.T, handler http.Handler, fields map[string]string, files []testUpload, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(part, strings.NewReader(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/queue-items", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func sessionCookieFrom(t *testing.T, resp *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range resp.Result().Cookies() {
		if cookie.Name == sessionCookieName && strings.TrimSpace(cookie.Value) != "" {
			return cookie
		}
	}
	t.Fatal("expected session cookie")
	return nil
}
