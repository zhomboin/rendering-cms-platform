package articles

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

type PublishedArticleLookup interface {
	GetArticleBySlug(context.Context, string) (dbgen.GetArticleBySlugRow, error)
	GetArticleByArticleName(context.Context, string) (dbgen.GetArticleByArticleNameRow, error)
}

func ValidPublishedIdentifier(identifier string) bool {
	return utf8.RuneCountInString(identifier) <= 128 && (ValidSlug(identifier) || ValidArticleName(identifier))
}

// ResolvePublishedArticle 优先已发布短链；短链未命中时再按英文名查找。
// 数据库故障不会被当作未命中，也不会回退到未发布内容。
func ResolvePublishedArticle(ctx context.Context, lookup PublishedArticleLookup, identifier string) (dbgen.GetArticleBySlugRow, string, error) {
	if !ValidPublishedIdentifier(identifier) {
		return dbgen.GetArticleBySlugRow{}, "", errInvalidArticleIdentifier
	}
	if ValidSlug(identifier) {
		row, err := lookup.GetArticleBySlug(ctx, identifier)
		if err == nil {
			return row, "slug", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return dbgen.GetArticleBySlugRow{}, "", err
		}
	}
	if !ValidArticleName(identifier) {
		return dbgen.GetArticleBySlugRow{}, "", pgx.ErrNoRows
	}
	row, err := lookup.GetArticleByArticleName(ctx, identifier)
	return dbgen.GetArticleBySlugRow(row), "articleName", err
}
