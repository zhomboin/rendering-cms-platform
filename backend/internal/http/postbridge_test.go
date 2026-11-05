package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/integrations/postbridge"
)

type stubPostBridge struct {
	configured        bool
	registrationCalls int
	activationCalls   int
	targetCalls       int
	targetError       error
}

func (s *stubPostBridge) SigningConfigured() bool { return s.configured }

func (s *stubPostBridge) Register(_ context.Context, token, name string, grant postbridge.RegistrationGrant) (postbridge.Registration, error) {
	s.registrationCalls++
	if token != "synthetic-invitation" || name != "CMS test" || grant.SourceID != "cms-test" || len(grant.Accounts) != 1 || grant.Accounts[0] != "account-test" {
		return postbridge.Registration{}, errors.New("unexpected registration parameters")
	}
	return postbridge.Registration{ID: "registration-test", Status: "pending"}, nil
}

func (s *stubPostBridge) Activate(context.Context) error {
	s.activationCalls++
	return nil
}

func (s *stubPostBridge) Targets(context.Context) ([]postbridge.Target, error) {
	s.targetCalls++
	return []postbridge.Target{{AccountID: "account-test"}}, s.targetError
}

func TestPostBridgeAdminRoutesRequireAdmin(t *testing.T) {
	client := &stubPostBridge{configured: true}
	router := NewRouter(WithJWTSecret("test-only"), WithAdminRoutes(PostBridgeRoutes(client, postbridge.RegistrationGrant{SourceID: "cms-test", Accounts: []string{"account-test"}}, nil)))
	for _, path := range []string{"connection", "targets", "registrations", "activate"} {
		method := http.MethodGet
		if path == "registrations" || path == "activate" {
			method = http.MethodPost
		}
		for _, tc := range []struct {
			role string
			want int
		}{{"", 401}, {"editor", 403}} {
			rec := postBridgeAdminRequest(t, router, method, path, tc.role, "")
			if rec.Code != tc.want {
				t.Errorf("%s as %q returned %d, want %d", path, tc.role, rec.Code, tc.want)
			}
		}
	}
	if client.registrationCalls+client.activationCalls+client.targetCalls != 0 {
		t.Fatal("unauthorized request reached PostBridge")
	}
}

func TestPostBridgeAdminConnectionFlow(t *testing.T) {
	client := &stubPostBridge{configured: true}
	router := NewRouter(WithJWTSecret("test-only"), WithAdminRoutes(PostBridgeRoutes(client, postbridge.RegistrationGrant{SourceID: "cms-test", Accounts: []string{"account-test"}}, nil)))
	for _, tc := range []struct {
		method, path, payload, field string
		want                         int
	}{
		{"GET", "connection", "", "signingConfigured", 200},
		{"POST", "registrations", `{"invitationToken":"synthetic-invitation","name":"CMS test"}`, "registrationId", 201},
		{"POST", "registrations", `{"invitationToken":"synthetic-invitation","name":"CMS test","sourceId":"untrusted"}`, "error", 400},
		{"POST", "activate", "", "items", 200},
		{"GET", "targets", "", "items", 200},
	} {
		rec := postBridgeAdminRequest(t, router, tc.method, tc.path, "admin", tc.payload)
		var payload map[string]json.RawMessage
		if rec.Code != tc.want || json.Unmarshal(rec.Body.Bytes(), &payload) != nil || payload[tc.field] == nil ||
			rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), "synthetic-invitation") {
			t.Errorf("%s: status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if client.registrationCalls != 1 || client.activationCalls != 1 || client.targetCalls != 2 {
		t.Fatalf("unexpected upstream counts: %+v", client)
	}
	client.targetError = &postbridge.UpstreamError{Status: 429, Code: "rate_limited", RetryAfter: "60"}
	rec := postBridgeAdminRequest(t, router, "GET", "targets", "admin", "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("rate limit response: %d, %v", rec.Code, rec.Header())
	}
}

func postBridgeAdminRequest(t *testing.T, router http.Handler, method, path, role, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/admin/postbridge/"+path, strings.NewReader(payload))
	if role != "" {
		token, err := auth.IssueToken("test-only", "test-user", role)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPostBridgeActivationRemainsActiveWhenTargetQueryFails(t *testing.T) {
	client := &stubPostBridge{
		configured:  true,
		targetError: &postbridge.UpstreamError{Status: http.StatusTooManyRequests, Code: "rate_limited", RetryAfter: "60"},
	}
	router := NewRouter(WithJWTSecret("test-only"), WithAdminRoutes(PostBridgeRoutes(client, postbridge.RegistrationGrant{SourceID: "cms-test", Accounts: []string{"account-test"}}, nil)))
	rec := postBridgeAdminRequest(t, router, http.MethodPost, "activate", "admin", "")
	var payload struct {
		Status          string `json:"status"`
		TargetsVerified *bool  `json:"targetsVerified"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &payload) != nil || payload.Status != "active" ||
		payload.TargetsVerified == nil || *payload.TargetsVerified || client.activationCalls != 1 || client.targetCalls != 1 {
		t.Fatalf("activation result: HTTP %d, %s", rec.Code, rec.Body.String())
	}
}

func TestPostBridgeAttemptRoutesRequireAdmin(t *testing.T) {
	client := &stubPostBridge{configured: true}
	router := NewRouter(
		WithJWTSecret("test-only"),
		WithAdminRoutes(PostBridgeRoutes(client, postbridge.RegistrationGrant{}, &postbridge.Service{})),
	)
	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/articles/test-article/postbridge/preparations"},
		{http.MethodGet, "/api/v1/admin/postbridge/attempts"},
		{http.MethodGet, "/api/v1/admin/postbridge/attempts/test-attempt"},
		{http.MethodPost, "/api/v1/admin/postbridge/attempts/test-attempt/refresh"},
		{http.MethodPost, "/api/v1/admin/postbridge/attempts/test-attempt/submit"},
		{http.MethodPost, "/api/v1/admin/postbridge/attempts/test-attempt/cancel"},
	} {
		req := httptest.NewRequest(route.method, route.path, nil)
		token, err := auth.IssueToken("test-only", "test-user", "editor")
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s returned %d, want 403", route.method, route.path, rec.Code)
		}
	}
}

func TestUnsignedPostBridgeDoesNotMountAttemptRoutes(t *testing.T) {
	client := &stubPostBridge{}
	router := NewRouter(
		WithJWTSecret("test-only"),
		WithAdminRoutes(PostBridgeRoutes(client, postbridge.RegistrationGrant{}, &postbridge.Service{})),
	)
	for _, path := range []string{"attempts", "attempts/test-attempt"} {
		rec := postBridgeAdminRequest(t, router, http.MethodGet, path, "admin", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("unsigned request to %s returned %d", path, rec.Code)
		}
	}
}
