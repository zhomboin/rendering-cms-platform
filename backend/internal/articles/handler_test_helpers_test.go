package articles

import (
	"context"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

type articleStoreStub struct {
	searchRows             []dbgen.SearchPublishedArticlesRow
	publishedRows          []dbgen.ListPublishedArticlesRow
	lastSearchQuery        string
	lastArticleNameLookup  string
	createArgs             []dbgen.CreateDraftArticleParams
	updateArgs             []dbgen.UpdateDraftArticleParams
	updatePrivateArgs      []dbgen.UpdatePrivateArticleParams
	publishArgs            []pgtype.UUID
	unpublishArgs          []pgtype.UUID
	markPrivateArgs        []pgtype.UUID
	restoreDraftArgs       []pgtype.UUID
	revisionRows           []dbgen.ListArticleRevisionsRow
	revisionDetail         dbgen.GetArticleRevisionRow
	lastRevisionLookup     dbgen.GetArticleRevisionParams
	revisionListCalls      int
	getByIDArticle         dbgen.GetArticleByIDRow
	getByIDErr             error
	publishErr             error
	unpublishErr           error
	markPrivateErr         error
	restoreDraftErr        error
	byArticleNameArticle   dbgen.GetArticleByArticleNameRow
	searchCalls            int
	slugLookupCalls        int
	articleNameLookupCalls int
	listError              error
}

func (s *articleStoreStub) SearchPublishedArticles(ctx context.Context, query string) ([]dbgen.SearchPublishedArticlesRow, error) {
	s.searchCalls++
	s.lastSearchQuery = strings.TrimSpace(query)
	return s.searchRows, nil
}

func (s *articleStoreStub) ListPublishedArticles(ctx context.Context) ([]dbgen.ListPublishedArticlesRow, error) {
	if s.listError != nil {
		return nil, s.listError
	}
	return s.publishedRows, nil
}

func (s *articleStoreStub) GetArticleBySlug(ctx context.Context, slug string) (dbgen.GetArticleBySlugRow, error) {
	s.slugLookupCalls++
	return dbgen.GetArticleBySlugRow{}, pgx.ErrNoRows
}

func (s *articleStoreStub) GetArticleByArticleName(ctx context.Context, articleName string) (dbgen.GetArticleByArticleNameRow, error) {
	s.articleNameLookupCalls++
	s.lastArticleNameLookup = articleName
	if s.byArticleNameArticle.ArticleID.Valid {
		return s.byArticleNameArticle, nil
	}
	return dbgen.GetArticleByArticleNameRow{}, pgx.ErrNoRows
}

func (s *articleStoreStub) GetArticleByID(ctx context.Context, articleID pgtype.UUID) (dbgen.GetArticleByIDRow, error) {
	if s.getByIDErr != nil {
		return dbgen.GetArticleByIDRow{}, s.getByIDErr
	}
	if s.getByIDArticle.ArticleID.Valid {
		return s.getByIDArticle, nil
	}
	return articleByIDForTest(articleID, "aB3dE9", "短链文章"), nil
}

func (s *articleStoreStub) ListAdminArticles(ctx context.Context) ([]dbgen.ListAdminArticlesRow, error) {
	return nil, nil
}

func (s *articleStoreStub) CreateDraftArticle(ctx context.Context, arg dbgen.CreateDraftArticleParams) (dbgen.CreateDraftArticleRow, error) {
	s.createArgs = append(s.createArgs, arg)
	return createArticleForTest(uuidForTest("11111111-1111-1111-1111-111111111111"), arg.Slug, arg.ArticleName, arg.Title), nil
}

func (s *articleStoreStub) UpdateDraftArticle(ctx context.Context, arg dbgen.UpdateDraftArticleParams) (dbgen.UpdateDraftArticleRow, error) {
	s.updateArgs = append(s.updateArgs, arg)
	return updateArticleForTest(arg.ArticleID, arg.Slug, arg.ArticleName, arg.Title), nil
}

func (s *articleStoreStub) PublishArticle(ctx context.Context, articleID pgtype.UUID) (dbgen.PublishArticleRow, error) {
	if s.publishErr != nil {
		return dbgen.PublishArticleRow{}, s.publishErr
	}
	s.publishArgs = append(s.publishArgs, articleID)
	return publishArticleForTest(articleID, "aB3dE9"), nil
}

func (s *articleStoreStub) UnpublishArticle(ctx context.Context, articleID pgtype.UUID) (dbgen.UnpublishArticleRow, error) {
	if s.unpublishErr != nil {
		return dbgen.UnpublishArticleRow{}, s.unpublishErr
	}
	s.unpublishArgs = append(s.unpublishArgs, articleID)
	return unpublishArticleForTest(articleID, "aB3dE9"), nil
}

func (s *articleStoreStub) MarkArticlePrivate(ctx context.Context, articleID pgtype.UUID) (dbgen.MarkArticlePrivateRow, error) {
	if s.markPrivateErr != nil {
		return dbgen.MarkArticlePrivateRow{}, s.markPrivateErr
	}
	s.markPrivateArgs = append(s.markPrivateArgs, articleID)
	return markPrivateArticleForTest(articleID, "aB3dE9"), nil
}

func (s *articleStoreStub) RestoreArticleToDraft(ctx context.Context, articleID pgtype.UUID) (dbgen.RestoreArticleToDraftRow, error) {
	if s.restoreDraftErr != nil {
		return dbgen.RestoreArticleToDraftRow{}, s.restoreDraftErr
	}
	s.restoreDraftArgs = append(s.restoreDraftArgs, articleID)
	return restoreDraftArticleForTest(articleID, "aB3dE9"), nil
}

func (s *articleStoreStub) UpdatePrivateArticle(ctx context.Context, arg dbgen.UpdatePrivateArticleParams) (dbgen.UpdatePrivateArticleRow, error) {
	s.updatePrivateArgs = append(s.updatePrivateArgs, arg)
	return updatePrivateArticleForTest(arg.ArticleID, arg.Slug, arg.ArticleName, arg.Title), nil
}

func (s *articleStoreStub) ListArticleRevisions(ctx context.Context, articleID pgtype.UUID) ([]dbgen.ListArticleRevisionsRow, error) {
	s.revisionListCalls++
	return s.revisionRows, nil
}

func (s *articleStoreStub) GetArticleRevision(ctx context.Context, arg dbgen.GetArticleRevisionParams) (dbgen.GetArticleRevisionRow, error) {
	s.lastRevisionLookup = arg
	if s.revisionDetail.ArticleID.Valid {
		return s.revisionDetail, nil
	}
	return dbgen.GetArticleRevisionRow{}, pgx.ErrNoRows
}

func publishArticleForTest(articleID pgtype.UUID, slug string) dbgen.PublishArticleRow {
	return dbgen.PublishArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: "redis-sentinel-with-docker",
		Title:       "短链文章",
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusPublished,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func unpublishArticleForTest(articleID pgtype.UUID, slug string) dbgen.UnpublishArticleRow {
	return dbgen.UnpublishArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: "redis-sentinel-with-docker",
		Title:       "短链文章",
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func markPrivateArticleForTest(articleID pgtype.UUID, slug string) dbgen.MarkArticlePrivateRow {
	return dbgen.MarkArticlePrivateRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: "redis-sentinel-with-docker",
		Title:       "短链文章",
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusPrivate,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func restoreDraftArticleForTest(articleID pgtype.UUID, slug string) dbgen.RestoreArticleToDraftRow {
	return dbgen.RestoreArticleToDraftRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: "redis-sentinel-with-docker",
		Title:       "短链文章",
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func uuidForTest(value string) pgtype.UUID {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		panic(err)
	}
	return uuid
}

func articleByIDForTest(articleID pgtype.UUID, slug string, title string) dbgen.GetArticleByIDRow {
	return dbgen.GetArticleByIDRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: "redis-sentinel-with-docker",
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func createArticleForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.CreateDraftArticleRow {
	return dbgen.CreateDraftArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func updateArticleForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.UpdateDraftArticleRow {
	return dbgen.UpdateDraftArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func updatePrivateArticleForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.UpdatePrivateArticleRow {
	return dbgen.UpdatePrivateArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusPrivate,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func publishedArticleByNameForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.GetArticleByArticleNameRow {
	return dbgen.GetArticleByArticleNameRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusPublished,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func authenticatedArticleRouter(t *testing.T, handler Handler) chi.Router {
	t.Helper()
	router := chi.NewRouter()
	router.Use(httpapi.AdminAuthMiddleware("test-secret"))
	handler.RegisterAdminRoutes(router)
	return router
}

func adminToken(t *testing.T) string {
	t.Helper()
	token, err := auth.IssueToken("test-secret", "22222222-2222-2222-2222-222222222222", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return token
}
