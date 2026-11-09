package comments

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestCommentIdentifiersMatchPublicArticleResolution(t *testing.T) {
	pool := testutil.Database(t)
	ctx := context.Background()
	testutil.Exec(t, pool, "insert into users(user_id,email,name,password_hash) values('11111111-1111-1111-1111-111111111111','fixture@example.invalid','测试','synthetic')")
	testutil.Exec(t, pool, "insert into articles(slug,article_name,title,summary,body_mdx,status,author_id) values('aB3dE9','docker','测试文章','','正文','published','11111111-1111-1111-1111-111111111111')")
	q := dbgen.New(pool)
	create := func() (dbgen.Comment, error) {
		return q.CreateComment(ctx, dbgen.CreateCommentParams{
			Slug: "docker", IsSlug: true, AuthorName: "测试", Body: "评论",
			IpHash: "synthetic-hash", AuthorEmail: pgtype.Text{},
		})
	}
	english, err := create()
	if err != nil {
		t.Fatal("six-character English name did not resolve", err)
	}
	article, via, err := articles.ResolvePublishedArticle(ctx, q, "docker")
	if err != nil || article.ArticleID != english.ArticleID || via != "articleName" {
		t.Fatal("comment and public detail disagreed on English name")
	}
	testutil.Exec(t, pool, "insert into articles(slug,article_name,title,summary,body_mdx,status,author_id) values('docker','other-name','短链文章','','正文','published','11111111-1111-1111-1111-111111111111')")
	slug, err := create()
	if err != nil || slug.ArticleID == english.ArticleID {
		t.Fatal("published short slug did not take priority", err)
	}
	article, via, err = articles.ResolvePublishedArticle(ctx, q, "docker")
	if err != nil || article.ArticleID != slug.ArticleID || via != "slug" {
		t.Fatal("comment and public detail disagreed on slug priority")
	}
	for _, id := range []pgtype.UUID{english.CommentID, slug.CommentID} {
		if _, err := q.ReviewComment(ctx, dbgen.ReviewCommentParams{CommentID: id, Status: "approved"}); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := q.ListApprovedCommentsByArticleSlug(ctx, dbgen.ListApprovedCommentsByArticleSlugParams{Slug: "docker", IsSlug: true})
	if err != nil || len(listed) != 1 || listed[0].CommentID != slug.CommentID {
		t.Fatal("comments mixed articles sharing an identifier", err)
	}
	testutil.Exec(t, pool, "update articles set status='private'")
	if _, err := create(); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("comment accepted an unpublished article", err)
	}
	listed, err = q.ListApprovedCommentsByArticleSlug(ctx, dbgen.ListApprovedCommentsByArticleSlugParams{Slug: "docker", IsSlug: true})
	if err != nil || len(listed) != 0 {
		t.Fatal("private article comments were exposed")
	}
}
