package articlemcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
	"rendering-cms-platform/backend/internal/testutil"
)

type tokenTransport string

func (t tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+string(t))
	return http.DefaultTransport.RoundTrip(req)
}
func TestMCPThroughHTTP(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	var owner pgtype.UUID
	if err := pool.QueryRow(ctx, "insert into users(email,name,password_hash) values ('mcp@example.invalid','测试','unused') returning user_id").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	blog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("凭据被转发至博客")
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<head><link rel=canonical href=%q></head>", r.URL.Path)
	}))
	t.Cleanup(blog.Close)
	service := New(pool, config.ArticleMCPConfig{BlogOrigin: blog.URL, AdminOrigin: "https://cms.example.invalid", Origin: "https://mcp.example.invalid"})
	const secret = "test-only-signing-secret"
	host := httptest.NewServer(httpapi.NewRouter(httpapi.WithJWTSecret(secret), httpapi.WithPublicRoutes(service.Register), httpapi.WithAdminRoutes(service.RegisterAdmin), httpapi.WithAdminRoutes(articles.NewHandler(dbgen.New(pool)).RegisterAdminRoutes)))
	t.Cleanup(host.Close)
	admin, _ := auth.IssueToken(secret, owner.String(), "admin")
	editor, _ := auth.IssueToken(secret, owner.String(), "editor")
	request := func(method, path, token, body string, want int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, host.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != want {
			t.Fatalf("%s %s: %d (%s)", method, path, resp.StatusCode, raw)
		}
		return raw
	}
	connections := "/api/v1/admin/mcp/connections"
	request("POST", endpoint, "", "{}", 401)
	request("POST", endpoint, admin, "{}", 401)
	request("POST", connections, editor, "{}", 403)
	request("POST", connections, admin, "{}", 400)
	type grant struct{ ID, Token string }
	issue := func() grant {
		var g grant
		if err := json.Unmarshal(request("POST", connections, admin, `{"label":"写作"}`, 201), &g); err != nil || g.Token == "" || g.ID == "" {
			t.Fatalf("连接凭据 %v", err)
		}
		return g
	}
	g := issue()
	request("GET", connections, g.Token, "", 401)
	if bytes.Contains(request("GET", connections, admin, "", 200), []byte(g.Token)) {
		t.Fatal("列表泄露凭据")
	}
	connect := func(token string) *mcp.ClientSession {
		c := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "1"}, nil)
		session, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: host.URL + endpoint, HTTPClient: &http.Client{Transport: tokenTransport(token)}, DisableStandaloneSSE: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { session.Close() })
		return session
	}
	session := connect(g.Token)
	call := func(s *mcp.ClientSession, name string, in any, wantError bool, out any) {
		t.Helper()
		res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: in})
		if err != nil {
			if wantError {
				return
			}
			t.Fatal(err)
		}
		if res.IsError != wantError {
			t.Fatalf("%s error=%v: %v", name, res.IsError, res.Content)
		}
		if !wantError && out != nil {
			b, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(b, out); err != nil {
				t.Fatal(err)
			}
		}
	}
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 4 {
		t.Fatalf("草稿工具发现 %v, %v", list, err)
	}
	in := CreateInput{RequestKey: "weekly-1", ArticleName: "weekly", Content: Content{Title: "周报", Body: "初稿"}}
	var first, again Article
	call(session, "create_draft", in, false, &first)
	call(session, "create_draft", in, false, &again)
	if first.ArticleID == "" || first.ArticleID != again.ArticleID || first.Version != 1 || first.Status != "draft" || !strings.HasSuffix(first.PreviewURL, "/read") {
		t.Fatal("建稿或幂等契约", first)
	}
	in.Title = "重复键的新正文"
	call(session, "create_draft", in, true, nil)
	other := connect(issue().Token)
	article := ArticleInput{first.ArticleID}
	call(other, "get_article", article, true, nil)
	action := VersionInput{ArticleID: first.ArticleID, ExpectedVersion: first.Version}
	call(session, "publish_article", action, true, nil)
	edit := EditInput{VersionInput: action, Content: Content{Title: "修订", Body: "修订正文"}}
	call(session, "update_draft", edit, false, &again)
	call(session, "update_draft", edit, true, nil)
	manualPath := "/api/v1/admin/articles/" + first.ArticleID
	request("PATCH", manualPath+"?expectedVersion=1", admin, `{"title":"过期编辑","articleName":"weekly","bodyMdx":"正文"}`, 409)
	request("POST", manualPath+"/publish?expectedVersion=1", admin, "", 409)
	request("POST", manualPath+"/publish?expectedVersion=-1", admin, "", 400)
	request("PATCH", connections+"/"+g.ID, admin, `{"allowPublish":true}`, 204)
	call(session, "publish_article", action, true, nil)
	action.ExpectedVersion = again.Version
	call(session, "publish_article", action, false, &again)
	if again.Status != "published" || again.PublicURL != blog.URL+"/blog/"+first.Slug {
		t.Fatal("发布结果", again)
	}
	var status Visibility
	call(session, "article_status", article, false, &status)
	if status.Page != "reachable" || status.Search != "not_checked" {
		t.Fatal(status)
	}
	check := "/api/v1/admin/mcp/articles/" + first.ArticleID + "/search-check"
	request("GET", check, admin, "", 200)
	request("POST", check, admin, `{"version":1,"result":"found"}`, 409)
	request("POST", check, admin, fmt.Sprintf(`{"version":%d,"result":"found"}`, again.Version), 204)
	call(session, "article_status", article, false, &status)
	if status.Search != "found" || status.SearchCheckedAt == "" {
		t.Fatal(status)
	}
	testutil.Exec(t, pool, "update articles set status='draft' where article_id=$1", first.ArticleID)
	testutil.Exec(t, pool, "update articles set status='private' where article_id=$1", first.ArticleID)
	call(session, "get_article", article, true, nil)
	testutil.Exec(t, pool, "update articles set status='draft' where article_id=$1", first.ArticleID)
	call(session, "get_article", article, true, nil)
	request("DELETE", connections+"/"+g.ID, admin, "", 204)
	request("POST", endpoint, g.Token, "{}", 401)
}

func TestConcurrentVersions(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	var owner pgtype.UUID
	if err := pool.QueryRow(ctx, "insert into users(email,name,password_hash) values ('author@example.invalid','测试','unused') returning user_id").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("test"))
	q := dbgen.New(pool)
	id, err := q.CreateMCPConnection(ctx, dbgen.CreateMCPConnectionParams{OwnerID: owner, TokenHash: hash[:], Label: "写作"})
	if err != nil {
		t.Fatal(err)
	}
	p := principal{id: id}
	s := store{pool: pool}
	in := CreateInput{RequestKey: "article", ArticleName: "article", Content: Content{Title: "初稿", Body: "正文"}}
	type outcome struct {
		a   Article
		err error
	}
	results := make(chan outcome, 2)
	for range 2 {
		go func() { a, e := s.create(ctx, p, in); results <- outcome{a, e} }()
	}
	one, two := <-results, <-results
	if one.err != nil || two.err != nil || one.a.ArticleID != two.a.ArticleID {
		t.Fatalf("并发建稿 %v %v", one, two)
	}
	for range 2 {
		go func() {
			content := Content{Title: "并发改稿", Body: "正文"}
			a, e := s.change(ctx, p, VersionInput{one.a.ArticleID, one.a.Version}, &content)
			results <- outcome{a, e}
		}()
	}
	a, b := <-results, <-results
	if !((a.err == nil && errors.Is(b.err, errVersion)) || (b.err == nil && errors.Is(a.err, errVersion))) {
		t.Fatalf("并发改稿 %v %v", a, b)
	}
}
