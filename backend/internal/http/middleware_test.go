package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rendering-cms-platform/backend/internal/auth"
)

func TestAdminAuthMiddlewareRequiresBearerToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/articles", nil)
	rec := httptest.NewRecorder()

	AdminAuthMiddleware("secret-32-characters-minimum-value")(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminAuthMiddlewareAcceptsAdminToken(t *testing.T) {
	token, err := auth.IssueToken("secret-32-characters-minimum-value", "user-1", "admin")
	if err != nil {
		t.Fatalf("IssueToken() returned error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			t.Fatal("expected user in context")
		}
		if user.UserID != "user-1" || user.Role != "admin" {
			t.Fatalf("unexpected user: %#v", user)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	AdminAuthMiddleware("secret-32-characters-minimum-value")(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestClientIPHashIgnoresSpoofedForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = "10.0.0.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.10")

	spoofedHash := ClientIPHash(req)

	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "203.0.113.10")
	if realIPHash := ClientIPHash(req); realIPHash == spoofedHash {
		t.Fatalf("spoofed X-Forwarded-For hash %q should not match trusted X-Real-IP hash", spoofedHash)
	}

	req.Header.Del("X-Real-IP")
	if remoteHash := ClientIPHash(req); remoteHash != spoofedHash {
		t.Fatalf("remote address hash = %q, want spoofed X-Forwarded-For to be ignored", remoteHash)
	}
}

func TestCORSMiddlewareRejectsUnlistedRequestHeaders(t *testing.T) {
	handler := CORSMiddleware([]string{"http://127.0.0.1:5173"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-Evil-Header")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Headers"); strings.Contains(got, "X-Evil-Header") || strings.Contains(got, "*") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want no unlisted header echo", got)
	}
}

func TestRequestLogMiddlewareLogsClientIPHashInsteadOfRawIP(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := RequestLogMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	line := logs.String()
	if strings.Contains(line, `"remote_addr":"192.0.2.10"`) {
		t.Fatalf("log line %q should not contain raw client IP", line)
	}
	if !strings.Contains(line, `"remote_ip_hash":`) {
		t.Fatalf("log line %q should contain remote_ip_hash", line)
	}
}

func TestRequestSizeLimitMiddlewareRejectsOversizedBody(t *testing.T) {
	handler := RequestSizeLimitMiddleware(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Body.Read(make([]byte, 8)); err == nil {
			t.Fatal("expected request body read to fail after limit")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", strings.NewReader("too-large"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
