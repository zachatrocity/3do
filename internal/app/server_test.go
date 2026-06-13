package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	memberCookie := sessionCookieFrom(t, resp)

	resp = requestJSON(t, handler, http.MethodGet, "/api/users", nil, memberCookie)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected member user listing to be forbidden, got %d: %s", resp.Code, resp.Body.String())
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
	return NewServer(config.Config{
		AppURL:        "http://example.test",
		DataDir:       t.TempDir(),
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
