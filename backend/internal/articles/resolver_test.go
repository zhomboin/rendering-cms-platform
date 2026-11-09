package articles

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

type resolverStore struct {
	*articleStoreStub
	slugRow   dbgen.GetArticleBySlugRow
	slugError error
}

func (s resolverStore) GetArticleBySlug(ctx context.Context, identifier string) (dbgen.GetArticleBySlugRow, error) {
	return s.slugRow, s.slugError
}

func TestResolveSixCharacterNamesAndSlugPriority(t *testing.T) {
	id := uuidForTest("11111111-1111-1111-1111-111111111111")
	other := uuidForTest("22222222-2222-2222-2222-222222222222")
	for _, name := range []string{"docker", "golang", "abc123"} {
		store := &articleStoreStub{
			byArticleNameArticle: publishedArticleByNameForTest(id, "aB3dE9", name, "测试文章"),
		}
		row, via, err := ResolvePublishedArticle(context.Background(), store, name)
		if err != nil || row.ArticleID != id || via != "articleName" || store.articleNameLookupCalls != 1 {
			t.Fatalf("name=%s via=%s error=%v", name, via, err)
		}
		priority := resolverStore{articleStoreStub: store, slugRow: dbgen.GetArticleBySlugRow{ArticleID: other}}
		row, via, err = ResolvePublishedArticle(context.Background(), priority, name)
		if err != nil || row.ArticleID != other || via != "slug" || store.articleNameLookupCalls != 1 {
			t.Fatal("published slug must win without querying the English name")
		}
	}
}

func TestResolveDoesNotHideDatabaseFailuresOrGuessInvalidNames(t *testing.T) {
	failed := errors.New("database unavailable")
	store := resolverStore{articleStoreStub: &articleStoreStub{}, slugError: failed}
	if _, _, err := ResolvePublishedArticle(context.Background(), store, "docker"); !errors.Is(err, failed) {
		t.Fatal("database failure was hidden")
	}
	if store.articleNameLookupCalls != 0 {
		t.Fatal("database failure must not trigger fallback")
	}
	store.slugError = pgx.ErrNoRows
	if _, _, err := ResolvePublishedArticle(context.Background(), store, "ABC123"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("uppercase short slug must not be tried as an English name")
	}
}
