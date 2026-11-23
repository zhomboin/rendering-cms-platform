package articlemcp

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"rendering-cms-platform/backend/internal/config"
)

func TestOAuthAccessToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.ArticleMCPConfig{Origin: "https://cms.example.invalid", Issuer: "https://identity.example.invalid", Key: &key.PublicKey}
	service := New(nil, cfg)
	for _, tc := range []struct {
		name, issuer, audience, typ string
		until                       time.Duration
		valid                       bool
	}{
		{"授权", cfg.Issuer, cfg.Origin + endpoint, "at+jwt", time.Hour, true},
		{"过期", cfg.Issuer, cfg.Origin + endpoint, "at+jwt", -time.Hour, false},
		{"其他服务", cfg.Issuer, cfg.Origin, "at+jwt", time.Hour, false},
		{"其他发行方", cfg.Origin, cfg.Origin + endpoint, "at+jwt", time.Hour, false},
		{"身份令牌", cfg.Issuer, cfg.Origin + endpoint, "JWT", time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "subject", Issuer: tc.issuer, Audience: jwt.ClaimStrings{tc.audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(tc.until))}, ClientID: "writer", Scope: draftScope})
			tok.Header["typ"] = tc.typ
			raw, err := tok.SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.oauthClaims(raw)
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
		})
	}
	r := chi.NewRouter()
	service.registerEnabled(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", metadata, nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("POST", endpoint, nil))
	if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal(rec.Result())
	}
	req := httptest.NewRequest("POST", endpoint, nil)
	req.Header.Set("Origin", "https://elsewhere.example.invalid")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal(rec.Code)
	}
}

func TestPageProbe(t *testing.T) {
	for _, tc := range []struct {
		code       int
		body, want string
	}{
		{200, "<link rel='canonical' href='/blog/abc123'>", "reachable"},
		{200, "<link rel='canonical' href='/'>", "unverified"},
		{200, "<html><body>错误页面</body></html>", "unverified"},
		{404, "", "not_found"}, {302, "", "unverified"}, {503, "", "unverified"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte(tc.body))
		}))
		svc := New(nil, config.ArticleMCPConfig{})
		got := svc.probe(t.Context(), srv.URL+"/blog/abc123")
		srv.Close()
		if got != tc.want {
			t.Fatalf("%d %q: %s", tc.code, tc.body, got)
		}
	}
}

func TestConfigurableBlogProbe(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "::1", "fd00::1", "64:ff9b::a00:1", "2002::1"} {
		if publicBlogIP(netip.MustParseAddr(addr)) {
			t.Fatalf("不应探测 %s", addr)
		}
	}
	if !publicBlogIP(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("公网地址不应被拒绝")
	}
}
