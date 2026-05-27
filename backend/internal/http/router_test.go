package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNewRouterExposesHealthEndpoint(t *testing.T) {
	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}

func TestNewRouterExposesRefreshEndpoint(t *testing.T) {
	router := NewRouter(WithRefreshHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestNewRouterAddsCORSHeadersForConfiguredFrontendOrigin(t *testing.T) {
	router := NewRouter(WithFrontendOrigin("http://127.0.0.1:5173"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestNewRouterAddsCORSHeadersForConfiguredFrontendOrigins(t *testing.T) {
	router := NewRouter(WithFrontendOrigins([]string{
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
	}))

	for _, origin := range []string{
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, origin)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8081")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for unconfigured origin", got)
	}
}

func TestNewRouterHandlesCORSPreflightForConfiguredFrontendOrigin(t *testing.T) {
	router := NewRouter(WithFrontendOrigins([]string{
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
	}), WithPublicRoutes(func(router chi.Router) {
		router.Post("/api/v1/articles/{slug}/views", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/articles/test-slug/views", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want to include POST", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want to include Content-Type", got)
	}
}

func TestNewRouterLogsRequestSummary(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := NewRouter(WithLogger(logger))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	req.Header.Set("User-Agent", "router-test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	line := logs.String()
	for _, want := range []string{
		`"msg":"http_request"`,
		`"method":"GET"`,
		`"path":"/api/v1/health"`,
		`"status":200`,
		`"remote_ip_hash":`,
		`"user_agent":"router-test"`,
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("log line %q does not contain %s", line, want)
		}
	}
}
