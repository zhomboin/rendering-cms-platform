package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestNewRouterAppliesSeparatePublicReadAndSearchLimits(t *testing.T) {
	router := NewRouter(
		WithPublicTrafficLimits(PublicTrafficLimits{
			ReadRatePerSecond: 1, ReadBurst: 1,
			SearchRatePerSecond: 1, SearchBurst: 1,
			MaxInFlight: 4, MaxClients: 10, ClientTTL: time.Minute,
		}),
		WithPublicArticleReadRoutes(func(router chi.Router) {
			router.Get("/api/v1/articles", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
			router.Get("/api/v1/articles/{slug}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) })
		}),
		WithPublicArticleSearchRoutes(func(router chi.Router) {
			router.Get("/api/v1/articles/search", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
		}),
	)

	request := func(path string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := request("/api/v1/articles"); got != http.StatusNoContent {
		t.Fatalf("first read status = %d", got)
	}
	if got := request("/api/v1/articles"); got != http.StatusTooManyRequests {
		t.Fatalf("second read status = %d, want 429", got)
	}
	if got := request("/api/v1/articles/search"); got != http.StatusNoContent {
		t.Fatalf("search should use an independent bucket; status = %d", got)
	}
	if got := request("/api/v1/health"); got != http.StatusOK {
		t.Fatalf("health must not use public limiter; status = %d", got)
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
