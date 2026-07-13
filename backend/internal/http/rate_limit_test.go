package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRateLimiterEnforcesBurstAndRefill(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	limiter := NewClientRateLimiter(ClientRateLimitOptions{
		RatePerSecond: 2, Burst: 2, MaxClients: 10, ClientTTL: time.Minute,
		Now: func() time.Time { return now },
	})
	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("burst request should be allowed")
	}
	if allowed, retryAfter := limiter.Allow("client-a"); allowed || retryAfter <= 0 {
		t.Fatalf("third request = (%v, %v), want denied with retry delay", allowed, retryAfter)
	}
	if allowed, _ := limiter.Allow("client-b"); !allowed {
		t.Fatal("different client should have an independent bucket")
	}
	now = now.Add(500 * time.Millisecond)
	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("one token should refill after 500ms at 2 requests/second")
	}
}

func TestClientRateLimiterBoundsAndExpiresClientState(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	limiter := NewClientRateLimiter(ClientRateLimitOptions{
		RatePerSecond: 1, Burst: 1, MaxClients: 2, ClientTTL: time.Minute,
		Now: func() time.Time { return now },
	})
	limiter.Allow("client-a")
	limiter.Allow("client-b")
	for _, client := range []string{"client-c", "client-d", "client-e"} {
		limiter.Allow(client)
	}
	if got := limiter.ClientCount(); got != 2 {
		t.Fatalf("client count = %d, want hard maximum 2", got)
	}
	if allowed, _ := limiter.Allow("client-f"); allowed {
		t.Fatal("overflow clients should share the exhausted overflow bucket")
	}

	now = now.Add(2 * time.Minute)
	limiter.Allow("client-new")
	if got := limiter.ClientCount(); got != 1 {
		t.Fatalf("client count after TTL cleanup = %d, want 1", got)
	}
}

func TestRateLimitMiddlewareUsesTrustedClientAddress(t *testing.T) {
	limiter := NewClientRateLimiter(ClientRateLimitOptions{
		RatePerSecond: 1, Burst: 1, MaxClients: 10, ClientTTL: time.Minute,
	})
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(realIP, forwardedFor string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Real-IP", realIP)
		req.Header.Set("X-Forwarded-For", forwardedFor)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := request("198.51.100.1", "203.0.113.1"); got != http.StatusNoContent {
		t.Fatalf("first request status = %d", got)
	}
	if got := request("198.51.100.1", "203.0.113.2"); got != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Forwarded-For status = %d, want 429", got)
	}
	if got := request("198.51.100.2", "203.0.113.1"); got != http.StatusNoContent {
		t.Fatalf("different trusted X-Real-IP status = %d", got)
	}
}

func TestConcurrencyLimitMiddlewareRejectsWithoutWaiting(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := ConcurrencyLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	<-entered
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	close(release)
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second concurrent request status = %d, want 503", second.Code)
	}
}
