package articlemcp

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestCoverAddress(t *testing.T) {
	for _, good := range []string{"", "/images/a.png", "https://cdn.example.invalid/a.webp?width=800&fit=cover", "http://images.example.invalid/封面.jpg"} {
		if !validCoverURL(good) {
			t.Errorf("未接受封面 %q", good)
		}
	}
	for _, bad := range []string{"a.jpg", "//cdn.example.invalid/a", "javascript:void(0)", "https://user:pass@example.invalid/a", "https:///missing-host", "/a?X-Goog-Signature=x", "https://example.invalid/a?%58-Amz-Credential=x", "/a?Expires=1", "/a\\b", "/a\nb"} {
		if validCoverURL(bad) {
			t.Errorf("错误接受封面 %q", bad)
		}
	}
}

func TestCreateRequestHashInputWithoutPresentation(t *testing.T) {
	// 已签发的请求键按建稿 JSON 摘要去重，省略展示字段时须能重试原请求。
	input := CreateInput{RequestKey: "memo", ArticleName: "memo", Content: Content{Title: "标题", Summary: "摘要", Body: "正文", Tags: []string{"go"}}}
	raw, err := json.Marshal(input)
	const persisted = `{"requestKey":"memo","articleName":"memo","title":"标题","summary":"摘要","bodyMdx":"正文","tags":["go"]}`
	if err != nil || string(raw) != persisted {
		t.Fatalf("建稿摘要输入发生变化 %s: %v", raw, err)
	}
}

func checkPresentationTools(t *testing.T, client *mcp.ClientSession, permissions func(string), q *dbgen.Queries) string {
	t.Helper()
	ctx := t.Context()
	tools, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != createTool && tool.Name != editTool {
			continue
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any
			Required   []string
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"coverImageUrl", "featured", "featuredRank"} {
			if schema.Properties[key] == nil || slices.Contains(schema.Required, key) {
				t.Fatal(tool.Name, key, string(raw))
			}
		}
	}
	call := func(tool string, input any, fail bool) Article {
		t.Helper()
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: input})
		if err != nil {
			t.Fatal(tool, err)
		}
		if res.IsError != fail {
			t.Fatalf("%s: %v", tool, res.Content)
		}
		var article Article
		if !fail {
			raw, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &article); err != nil {
				t.Fatal(err)
			}
		}
		return article
	}
	base := CreateInput{ArticleName: "cover-test", RequestKey: "cover-test", Content: Content{Title: "封面测试", Body: "正文", Summary: "摘要", Tags: []string{"cover"}}}
	const cover = "https://images.example.invalid/cover.webp"
	base.Presentation = Presentation{CoverImageURL: new(cover), Featured: new(true), FeaturedRank: new(int32(1))}
	first := call(createTool, base, false)
	since := first.FeaturedAt
	if first.Featured == nil || !*first.Featured || first.FeaturedRank == nil || *first.FeaturedRank != 1 || first.CoverImageURL == nil || *first.CoverImageURL != cover || since == nil {
		t.Fatal(first)
	}
	replay := call(createTool, base, false)
	if first.ArticleID != replay.ArticleID || first.Version != replay.Version || replay.FeaturedAt == nil || !since.Equal(*replay.FeaturedAt) {
		t.Fatal(replay)
	}
	base.FeaturedRank = new(int32(999))
	call(createTool, base, true)
	ref := VersionInput{ArticleID: first.ArticleID, ExpectedVersion: first.Version}
	edit := EditInput{VersionInput: ref, Presentation: Presentation{Featured: new(false), CoverImageURL: new(""), FeaturedRank: new(int32(999))}}
	changed := call(editTool, edit, false)
	if changed.Version <= first.Version || changed.Title != base.Title || changed.Body != base.Body || changed.Summary != base.Summary || !slices.Equal(changed.Tags, base.Tags) {
		t.Fatal(changed)
	}
	fetched := call(readTool, ArticleInput{first.ArticleID}, false)
	if fetched.Featured == nil || *fetched.Featured || fetched.FeaturedRank == nil || *fetched.FeaturedRank != 999 || fetched.CoverImageURL == nil || *fetched.CoverImageURL != "" || fetched.FeaturedAt == nil || !since.Equal(*fetched.FeaturedAt) {
		t.Fatal(fetched)
	}
	call(editTool, edit, true)
	ref.ExpectedVersion = changed.Version
	for _, bad := range []EditInput{
		{VersionInput: ref},
		{VersionInput: ref, Title: new("缺正文")},
		{VersionInput: ref, Presentation: Presentation{FeaturedRank: new(int32(1000))}},
		{VersionInput: ref, Presentation: Presentation{FeaturedRank: new(int32(0))}},
		{VersionInput: ref, Presentation: Presentation{CoverImageURL: new("https://images.example.invalid/a?Signature=x")}},
	} {
		call(editTool, bad, true)
	}
	edit = EditInput{VersionInput: ref, Presentation: Presentation{Featured: new(true), CoverImageURL: new("/images/cover.png")}}
	changed = call(editTool, edit, false)
	ref.ExpectedVersion = changed.Version
	text := call(editTool, EditInput{VersionInput: ref, Title: new("改稿"), Body: new("新正文")}, false)
	if text.Summary != "" || len(text.Tags) != 0 || text.FeaturedRank == nil || *text.FeaturedRank != 999 || text.Featured == nil || !*text.Featured || text.CoverImageURL == nil || *text.CoverImageURL != "/images/cover.png" || text.FeaturedAt == nil || !since.Equal(*text.FeaturedAt) {
		t.Fatal(text)
	}
	revision, err := q.GetArticleRevision(ctx, dbgen.GetArticleRevisionParams{ArticleID: mustArticleID(t, first.ArticleID), Version: text.Version})
	if err != nil || !revision.Featured || revision.FeaturedRank != 999 || revision.CoverImageUrl.String != "/images/cover.png" {
		t.Fatal(revision, err)
	}
	permissions(`{"permissions":["create_draft","update_draft","publish_article"]}`)
	base.FeaturedRank = new(int32(1))
	ref.ExpectedVersion = text.Version
	for _, receipt := range []Article{call(createTool, base, false), call(publishTool, ref, false)} {
		if receipt.Featured != nil || receipt.FeaturedRank != nil || receipt.CoverImageURL != nil || receipt.FeaturedAt != nil {
			t.Fatal("缺少读取授权的回执暴露展示字段", receipt)
		}
	}
	ref.ExpectedVersion++
	call(editTool, EditInput{VersionInput: ref, Presentation: Presentation{Featured: new(false)}}, true)
	return first.Slug
}
