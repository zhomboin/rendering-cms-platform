package articlemcp

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestOAuthGrantAndMigration(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	var owner pgtype.UUID
	if err := pool.QueryRow(ctx, "insert into users(email,name,password_hash) values ('oauth@example.invalid','Owner','unused') returning user_id").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	c := config.ArticleMCPConfig{Origin: "https://cms.example.invalid", Issuer: "https://id.example.invalid", Key: &key.PublicKey}
	svc := New(pool, c)
	q := dbgen.New(pool)
	id, err := q.CreateMCPConnection(ctx, dbgen.CreateMCPConnectionParams{OwnerID: owner, Label: "OAuth", OauthIssuer: textValue(c.Issuer), OauthClient: textValue("client"), OauthSubject: textValue("subject"), DraftPermissions: draftTools})
	if err != nil {
		t.Fatal(err)
	}
	token := func(subject, scope string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: c.Issuer, Audience: jwt.ClaimStrings{c.Origin + endpoint}, Subject: subject, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, ClientID: "client", Scope: scope})
		tok.Header["typ"] = "at+jwt"
		signed, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	if _, err := svc.verify(ctx, token("other", draftScope), nil); !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("其他主体获得授权 %v", err)
	}
	all := token("subject", draftScope+" "+publishScope)
	check := func(tok string, publish bool) {
		t.Helper()
		info, err := svc.verify(ctx, tok, nil)
		if err != nil || info.UserID != id.String() || slices.Contains(info.Scopes, publishScope) != publish {
			t.Fatalf("授权范围 %v %v", info, err)
		}
		permissions, _ := info.Extra["permissions"].([]string)
		if !slices.Equal(permissions, allPermissions(draftTools, publish)) {
			t.Fatal("OAuth 有效工具与授权范围不一致", permissions)
		}
	}
	check(all, false)
	if _, err = q.ChangeMCPPublish(ctx, dbgen.ChangeMCPPublishParams{OwnerID: owner, ID: id, AllowPublish: true}); err != nil {
		t.Fatal(err)
	}
	check(all, true)
	check(token("subject", draftScope), false)
	a, err := svc.store.create(ctx, principal{id: id}, CreateInput{RequestKey: "check", ArticleName: "check", Content: Content{Title: "检查", Body: "正文"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.ChangeMCPPermissions(ctx, dbgen.ChangeMCPPermissionsParams{OwnerID: owner, ID: id, DraftPermissions: []string{getAlbumTool}}); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{draftScope, draftScope + " " + albumScope} {
		info, err := svc.verify(ctx, token("subject", scope), nil)
		if err != nil {
			t.Fatal(err)
		}
		names := info.Extra["permissions"].([]string)
		if slices.Contains(names, getAlbumTool) != (scope != draftScope) {
			t.Fatal("OAuth 相册范围不匹配", scope, names)
		}
	}
	for _, migration := range []string{"000024_mcp_published_uploads.down", "000023_mcp_read_scope.down", "000022_mcp_albums.down", "000021_mcp_permissions.down", "000020_mcp_console.down", "000019_article_mcp.down", "000019_article_mcp.up", "000020_mcp_console.up", "000021_mcp_permissions.up", "000022_mcp_albums.up", "000023_mcp_read_scope.up", "000024_mcp_published_uploads.up"} {
		raw, err := os.ReadFile("../../migrations/" + migration + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		testutil.Exec(t, pool, string(raw))
	}
	if _, err := q.GetArticleByID(ctx, mustArticleID(t, a.ArticleID)); err != nil {
		t.Fatalf("迁移往返丢失文章 %v", err)
	}
}
func mustArticleID(t *testing.T, raw string) pgtype.UUID {
	t.Helper()
	id, err := parseID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
