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
	"slices"
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
	path := connections + "/" + g.ID
	expectTools := func(want ...string) {
		t.Helper()
		list, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, tool := range list.Tools {
			got = append(got, tool.Name)
		}
		slices.Sort(got)
		want = slices.Clone(want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("工具列表 %v，预期 %v", got, want)
		}
	}
	lookup := ArticleInput{first.ArticleID}
	request("PATCH", path, admin, `{"permissions":["article_status"],"expectedRevision":1}`, 204)
	request("PATCH", path, admin, `{"permissions":["publish_article"],"expectedRevision":1}`, 409)
	request("PATCH", path, editor, `{"permissions":["publish_article"]}`, 403)
	for _, invalid := range []string{`{"permissions":null}`, `{"permissions":["unknown"]}`, `{"permissions":["get_article","get_article"]}`, `{"permissions":[],"allowPublish":true}`} {
		request("PATCH", path, admin, invalid, 400)
	}
	expectTools(statusTool)
	call(session, statusTool, lookup, false, nil)
	call(session, readTool, lookup, true, nil)
	call(session, createTool, in, true, nil)
	call(session, editTool, EditInput{VersionInput: VersionInput{first.ArticleID, first.Version}, Content: in.Content}, true, nil)
	call(session, publishTool, VersionInput{first.ArticleID, first.Version}, true, nil)
	// 直接验证已完成鉴权的在途调用仍需在数据库事务内复核。
	grantPrincipal := principal{id: mustArticleID(t, g.ID), publishScope: true}
	if _, err := service.store.create(ctx, grantPrincipal, in); !errors.Is(err, errDenied) {
		t.Fatal("建稿缺少事务内校验", err)
	}
	request("PATCH", path, admin, `{"permissions":["list_albums","upload_album_photo"],"expectedRevision":2}`, 204)
	expectTools(listAlbumsTool, uploadPhotoTool)
	call(session, listAlbumsTool, AlbumPage{}, false, nil)
	request("PATCH", path, admin, `{"permissions":[]}`, 204)
	expectTools()
	call(session, statusTool, lookup, true, nil)
	request("PATCH", path, admin, `{"allowPublish":true}`, 204)
	expectTools(publishTool)
	var writeGrant grant
	request("POST", connections, admin, `{"label":"范围无效","readScope":"*"}`, 400)
	issued := request("POST", connections, admin, `{"label":"只写","permissions":["create_draft"],"readScope":"all"}`, 201)
	if !bytes.Contains(issued, []byte(`"readScope":"all"`)) {
		t.Fatal("签发回执缺少数据范围")
	}
	if err := json.Unmarshal(issued, &writeGrant); err != nil {
		t.Fatal(err)
	}
	writer := connect(writeGrant.Token)
	var receipt Article
	call(writer, createTool, CreateInput{RequestKey: "write-only", ArticleName: "write-only", Content: in.Content}, false, &receipt)
	if receipt.ArticleID == "" || receipt.Body != "" || receipt.Title != "" {
		t.Fatal("只写回执暴露内容或缺少文章 ID")
	}
	call(writer, readTool, ArticleInput{receipt.ArticleID}, true, nil)
	request("DELETE", connections+"/"+writeGrant.ID, admin, "", 204)
	request("PATCH", path, admin, `{"permissions":["create_draft","get_article","article_status","update_draft"]}`, 204)
	expectTools(draftTools...)
	in.Title = "重复键的新正文"
	call(session, "create_draft", in, true, nil)
	otherGrant := issue()
	other := connect(otherGrant.Token)
	otherPath := connections + "/" + otherGrant.ID
	article := ArticleInput{first.ArticleID}
	call(other, "get_article", article, true, nil)
	request("PATCH", otherPath, admin, `{"readScope":null}`, 400)
	request("PATCH", otherPath, editor, `{"readScope":"all"}`, 403)
	request("PATCH", otherPath, admin, `{"readScope":"all","expectedRevision":1}`, 204)
	request("PATCH", otherPath, admin, `{"readScope":"own","expectedRevision":1}`, 409)
	call(other, readTool, article, false, &receipt)
	if receipt.Body != first.Body {
		t.Fatal("授权后未读到文章正文")
	}
	call(other, statusTool, article, false, nil)
	manual, err := dbgen.New(pool).CreateDraftArticle(ctx, dbgen.CreateDraftArticleParams{
		AuthorID: owner, Slug: "manual", ArticleName: "manual", Title: "后台文章", BodyMdx: "正文", Tags: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	manualArticle := ArticleInput{manual.ArticleID.String()}
	call(other, readTool, manualArticle, false, nil)
	testutil.Exec(t, pool, "update articles set status='private' where article_id=$1", manual.ArticleID)
	call(other, readTool, manualArticle, true, nil)
	request("PATCH", otherPath, admin, `{"readScope":"own","expectedRevision":2}`, 204)
	call(other, readTool, article, true, nil)
	request("PATCH", otherPath, admin, `{"permissions":["get_article","update_draft","publish_article"],"readScope":"all","expectedRevision":3}`, 204)
	call(other, statusTool, article, true, nil)
	action := VersionInput{ArticleID: first.ArticleID, ExpectedVersion: first.Version}
	call(session, "publish_article", action, true, nil)
	edit := EditInput{VersionInput: action, Content: Content{Title: "修订", Body: "修订正文"}}
	call(other, editTool, edit, true, nil)
	call(other, publishTool, action, true, nil)
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
	call(other, readTool, article, true, nil)
	request("PATCH", path, admin, `{"readScope":"all"}`, 204)
	call(session, "get_article", article, true, nil)
	testutil.Exec(t, pool, "update articles set status='draft' where article_id=$1", first.ArticleID)
	call(session, "get_article", article, true, nil)
	request("DELETE", connections+"/"+g.ID, admin, "", 204)
	call(other, readTool, article, false, nil)
	request("POST", endpoint, g.Token, "{}", 401)
	settingsPath := "/api/v1/admin/mcp/settings"
	operationsPath := "/api/v1/admin/mcp/operations"
	request("GET", settingsPath, editor, "", 403)
	request("GET", operationsPath, g.Token, "", 401)
	request("GET", operationsPath+"?before=bad", admin, "", 400)
	grants := request("GET", connections, admin, "", 200)
	if !bytes.Contains(grants, []byte("integration")) || bytes.Contains(grants, []byte(`"lastSeenAt":null`)) {
		t.Fatal("未记录已使用连接的客户端和最近访问时间")
	}
	raw := request("GET", operationsPath, admin, "", 200)
	var history struct{ Items []operationItem }
	if err := json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	var creations int
	for _, item := range history.Items {
		if item.Operation == "create_draft" && item.Outcome == "success" && item.Connection == "写作" {
			if item.ArticleID.String() != first.ArticleID {
				t.Fatal("创建记录缺少真实文章 ID")
			}
			creations++
		}
	}
	if creations != 2 {
		t.Fatal("建稿与幂等重试均应记入工具日志")
	}
	for _, expected := range []string{"create_draft", "publish_article", "failed", "success", first.ArticleID} {
		if !bytes.Contains(raw, []byte(expected)) {
			t.Fatalf("审计缺少 %s", expected)
		}
	}
	for _, private := range []string{g.Token, "初稿", "weekly-1", "bodyMdx"} {
		if bytes.Contains(raw, []byte(private)) {
			t.Fatal("审计包含凭据或正文")
		}
	}
	anotherAdmin, _ := auth.IssueToken(secret, "00000000-0000-0000-0000-000000000002", "admin")
	t.Run("已发布相册追加授权", func(t *testing.T) {
		checkUploadGrantAPI(t, pool, owner, request, admin, anotherAdmin)
	})
	if bytes.Contains(request("GET", operationsPath, anotherAdmin, "", 200), []byte("create_draft")) {
		t.Fatal("泄漏其他管理员的记录")
	}
	disabled := `{"revision":0,"enabled":false,"origin":"https://writer.example.invalid","blogOrigin":"https://blog.example.invalid"}`
	request("PATCH", settingsPath, editor, disabled, 403)
	request("PATCH", settingsPath, admin, `{"revision":0,"enabled":true}`, 400)
	request("PATCH", settingsPath, admin, disabled, 204)
	request("PATCH", settingsPath, admin, disabled, 409)
	request("POST", endpoint, "", "{}", 404)
	reqSettings := request("GET", settingsPath, admin, "", 200)
	if !bytes.Contains(reqSettings, []byte("https://writer.example.invalid/api/v1/mcp")) {
		t.Fatal("连接地址未更新")
	}
	request("PATCH", settingsPath, admin, `{"revision":1,"enabled":true,"origin":"https://writer.example.invalid","blogOrigin":"https://blog.example.invalid"}`, 204)
	request("POST", endpoint, "", "{}", 401)
}

func TestSettingsBootstrap(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	s := New(pool, config.ArticleMCPConfig{})
	before, err := s.current(ctx)
	if err != nil || before.Enabled || before.Origin != "" {
		t.Fatal(before, err)
	}
	q := dbgen.New(pool)
	change := dbgen.SaveMCPSettingsParams{Enabled: true, Origin: "https://cms.example.invalid", AdminOrigin: "https://cms.example.invalid", BlogOrigin: "https://blog.example.invalid"}
	if n, err := q.SaveMCPSettings(ctx, change); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	after, err := New(pool, config.ArticleMCPConfig{}).current(ctx)
	if err != nil || !after.Enabled || after.Origin != change.Origin || after.Revision != 1 {
		t.Fatal(after, err)
	}
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
	id, err := q.CreateMCPConnection(ctx, dbgen.CreateMCPConnectionParams{OwnerID: owner, TokenHash: hash[:], Label: "写作", DraftPermissions: draftTools})
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
