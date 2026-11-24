package articlemcp

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

const draftScope = "articles:draft"
const publishScope = "articles:publish"
const tokenPrefix = "rcms_mcp_"
const endpoint = "/api/v1/mcp"
const metadata = "/.well-known/oauth-protected-resource/api/v1/mcp"

type claims struct {
	jwt.RegisteredClaims
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

func textValue(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func (s *Server) verify(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	q := dbgen.New(s.store.pool)
	var info mcpauth.TokenInfo
	var id pgtype.UUID
	var allow bool
	var drafts []string
	var err error
	if strings.HasPrefix(token, tokenPrefix) {
		hash := sha256.Sum256([]byte(token))
		row, e := q.LookupMCPToken(ctx, hash[:])
		id, allow, err = row.ID, row.AllowPublish, e
		drafts = row.DraftPermissions
		info.Scopes = []string{draftScope, publishScope}
	} else {
		c, e := s.oauthClaims(token)
		if e != nil {
			return nil, e
		}
		info.Expiration = c.ExpiresAt.Time
		info.Scopes = strings.Fields(c.Scope)
		row, e := q.LookupMCPOAuth(ctx, dbgen.LookupMCPOAuthParams{OauthIssuer: textValue(c.Issuer), OauthClient: textValue(c.ClientID), OauthSubject: textValue(c.Subject)})
		id, allow, err = row.ID, row.AllowPublish, e
		drafts = row.DraftPermissions
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, mcpauth.ErrInvalidToken
	}
	if err != nil {
		return nil, errors.New("无法验证连接")
	}
	if !allow {
		info.Scopes = slices.DeleteFunc(info.Scopes, func(s string) bool { return s == publishScope })
	}
	info.UserID = id.String()
	info.Extra = map[string]any{"permissions": allPermissions(drafts, slices.Contains(info.Scopes, publishScope))}
	return &info, nil
}
func (s *Server) oauthClaims(token string) (claims, error) {
	var c claims
	if s.cfg.Key == nil {
		return c, mcpauth.ErrInvalidToken
	}
	t, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return s.cfg.Key, nil }, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(s.cfg.Origin+endpoint), jwt.WithIssuer(s.cfg.Issuer), jwt.WithExpirationRequired())
	if err != nil || !t.Valid || c.Subject == "" || c.ClientID == "" {
		return c, mcpauth.ErrInvalidToken
	}
	typ, _ := t.Header["typ"].(string)
	if !strings.EqualFold(typ, "at+jwt") && !strings.EqualFold(typ, "application/at+jwt") {
		return c, mcpauth.ErrInvalidToken
	}
	return c, nil
}
