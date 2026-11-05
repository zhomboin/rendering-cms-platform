package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSignPostBridgeContract(t *testing.T) {
	var fixture struct {
		Signature struct {
			Method      string `json:"method"`
			URL         string `json:"url"`
			Body        string `json:"body"`
			ClientID    string `json:"client_id"`
			KeyID       string `json:"key_id"`
			Environment string `json:"environment"`
			ServiceID   string `json:"service_id"`
			Secret      string `json:"fixture_secret_base64"`
			Timestamp   int64  `json:"timestamp"`
			Nonce       string `json:"nonce"`
			Digest      string `json:"content_digest"`
			Input       string `json:"signature_input"`
			Signature   string `json:"signature"`
		} `json:"signature"`
	}
	raw, err := os.ReadFile("testdata/integration-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	vector := fixture.Signature
	client, err := NewClient("https://bridge.example", Identity{
		ClientID: vector.ClientID, KeyID: vector.KeyID, Environment: vector.Environment,
		ServiceID: vector.ServiceID, Secret: vector.Secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(vector.Method, vector.URL, strings.NewReader(vector.Body))
	if err != nil {
		t.Fatal(err)
	}
	client.sign(req, []byte(vector.Body), time.Unix(vector.Timestamp, 0), vector.Nonce)
	for header, want := range map[string]string{
		"Content-Digest": vector.Digest, "Signature-Input": vector.Input, "Signature": vector.Signature,
	} {
		if got := req.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestRegisterActivateAndTargets(t *testing.T) {
	client := testClient(t)
	var nonces []string
	var seen int
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen++
		switch req.Method + " " + req.URL.Path {
		case "POST /api/integrations/v1/registrations":
			var got registrationInput
			if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.SourceID != "cms-test" || got.Environment != "test" || got.Accounts[0] != "account-test" ||
				got.Operations[0] != "save_draft" || len(got.Scopes) != 1 || got.Scopes[0] != "targets:read" || req.Header.Get("Content-Type") != "application/json" ||
				req.Header.Get("X-Integration-Invitation") != "synthetic-invitation" || req.Header.Get("Signature") != "" {
				t.Fatal("unexpected registration request")
			}
			return response(201, `{"registration_id":"registration-test","status":"pending"}`), nil
		case "POST /api/integrations/v1/clients/self/activate", "GET /api/integrations/v1/targets":
			const emptyDigest = "sha-256=:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=:"
			if req.ContentLength != 0 || req.Header.Get("Content-Digest") != emptyDigest ||
				req.Header.Get("Signature") == "" || req.Header.Get("X-Integration-Client") != "client-test" {
				t.Fatal("unexpected signed empty request")
			}
			input := req.Header.Get("Signature-Input")
			_, nonce, ok := strings.Cut(input, `;nonce="`)
			if !ok {
				t.Fatal("missing nonce")
			}
			nonces = append(nonces, nonce)
			if req.Method == http.MethodPost {
				return response(200, `{"status":"active"}`), nil
			}
			return response(200, `{"items":[{"platform":"csdn","account_id":"account-test","display_name":"测试账号","operations":["save_draft"],"body_formats":["markdown"]}]}`), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})
	ctx := context.Background()
	registration, err := client.Register(ctx, "synthetic-invitation", "CMS test", RegistrationGrant{SourceID: "cms-test", Accounts: []string{"account-test"}})
	if err != nil || registration.ID != "registration-test" || registration.Status != "pending" {
		t.Fatalf("registration = %+v, %v", registration, err)
	}
	if err := client.Activate(ctx); err != nil {
		t.Fatal(err)
	}
	targets, err := client.Targets(ctx)
	if err != nil || len(targets) != 1 || targets[0].AccountID != "account-test" {
		t.Fatalf("targets = %+v, %v", targets, err)
	}
	if seen != 3 || len(nonces) != 2 || nonces[0] == nonces[1] {
		t.Fatal("expected three requests with a fresh nonce on each signed request")
	}
}

func TestClientRejectsRedirectAndReportsRateLimit(t *testing.T) {
	client := testClient(t)
	calls := 0
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			redirect := response(302, "")
			redirect.Header.Set("Location", "https://other.example/receive")
			return redirect, nil
		}
		limited := response(429, `{"code":"rate_limited","message":"private upstream diagnostics"}`)
		limited.Header.Set("Retry-After", "60")
		return limited, nil
	})
	_, err := client.Targets(context.Background())
	var upstream *UpstreamError
	if !errors.As(err, &upstream) || upstream.Status != 302 || calls != 1 {
		t.Fatalf("redirect result = %v, calls = %d", err, calls)
	}
	_, err = client.Targets(context.Background())
	if !errors.As(err, &upstream) || upstream.Status != 429 || upstream.Code != "rate_limited" ||
		upstream.RetryAfter != "60" || strings.Contains(err.Error(), "private upstream diagnostics") {
		t.Fatalf("rate limit result = %v", err)
	}
}

func TestClientRejectsIncompleteCredentialsAndInvalidOrigin(t *testing.T) {
	for _, origin := range []string{"http://bridge.example", "https://bridge.example/path", "https://user@bridge.example", "https://bridge.example?query", "https://bridge.example/#fragment"} {
		if _, err := NewClient(origin, Identity{Environment: "test"}); err == nil {
			t.Errorf("accepted %q", origin)
		}
	}
	if _, err := NewClient("https://bridge.example", Identity{Environment: "test", Secret: "invalid"}); err == nil {
		t.Fatal("accepted incomplete identity")
	}
	client, err := NewClient("https://bridge.example", Identity{Environment: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Targets(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unsigned targets result = %v", err)
	}
}

func testClient(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient("https://bridge.example", Identity{
		ClientID: "client-test", KeyID: "key-test", Environment: "test", ServiceID: "bridge-test",
		Secret: "c3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3M=",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestRegisterRequestsConfiguredScopes(t *testing.T) {
	client := testClient(t)
	grant := RegistrationGrant{
		SourceID: "cms-test", Accounts: []string{"account-test"},
		Scopes: []string{"targets:read", "preparations:write"},
	}
	calls := 0
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		var input registrationInput
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if strings.Join(input.Scopes, ",") != "targets:read,preparations:write" {
			t.Fatalf("unexpected requested scopes: %v", input.Scopes)
		}
		return response(201, `{"registration_id":"registration-test","status":"pending"}`), nil
	})
	ctx := context.Background()
	if _, err := client.Register(ctx, "synthetic-invitation", "CMS test", grant); err != nil {
		t.Fatal(err)
	}
	grant.Scopes = []string{"invalid:scope"}
	if _, err := client.Register(ctx, "synthetic-invitation", "CMS test", grant); !errors.Is(err, ErrInvalidRegistration) || calls != 1 {
		t.Fatalf("invalid scope: %v, upstream calls = %d", err, calls)
	}
}
