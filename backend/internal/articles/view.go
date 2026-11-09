package articles

import (
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func mapArticles[T articleRow](articles []T) []map[string]interface{} {
	response := make([]map[string]interface{}, 0, len(articles))
	for _, article := range articles {
		response = append(response, mapArticle(article))
	}
	return response
}

func mapPublicArticleSummaries[T articleRow](articles []T) []map[string]interface{} {
	response := make([]map[string]interface{}, 0, len(articles))
	for _, article := range articles {
		response = append(response, mapPublicArticleSummary(articleViewFromRow(article)))
	}
	return response
}

func mapPublicArticleSummary(article articleView) map[string]interface{} {
	return map[string]interface{}{
		"slug":          article.Slug,
		"canonicalSlug": article.Slug,
		"articleName":   article.ArticleName,
		"title":         article.Title,
		"summary":       article.Summary,
		"tags":          article.Tags,
		"publishedAt":   timestamptzValue(article.PublishedAt),
		"updatedAt":     timestamptzValue(article.UpdatedAt),
		"isFeatured":    article.Featured,
		"featuredRank":  article.FeaturedRank,
		"featuredAt":    timestamptzValue(article.FeaturedAt),
		"coverImageUrl": textValue(article.CoverImageUrl),
	}
}

func mapPublicArticleDetail(article articleView) map[string]interface{} {
	response := mapPublicArticleSummary(article)
	response["bodyMdx"] = article.BodyMdx
	return response
}

type articleRow interface {
	dbgen.CreateDraftArticleRow |
		dbgen.GetArticleByIDRow |
		dbgen.GetArticleByArticleNameRow |
		dbgen.GetArticleBySlugRow |
		dbgen.GetArticleRevisionRow |
		dbgen.ListAdminArticlesRow |
		dbgen.ListPublishedArticlesRow |
		dbgen.MarkArticlePrivateRow |
		dbgen.PublishArticleRow |
		dbgen.RestoreArticleToDraftRow |
		dbgen.UnpublishArticleRow |
		dbgen.UpdateDraftArticleRow |
		dbgen.UpdatePrivateArticleRow |
		articleView
}

type articleView struct {
	ArticleID     pgtype.UUID
	Slug          string
	ArticleName   string
	Title         string
	Summary       string
	BodyMdx       string
	Status        dbgen.ArticleStatus
	Tags          []string
	Featured      bool
	FeaturedRank  int32
	FeaturedAt    pgtype.Timestamptz
	CoverImageUrl pgtype.Text
	PublishedAt   pgtype.Timestamptz
	AuthorID      pgtype.UUID
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
	Version       int32
}

func mapArticle[T articleRow](row T) map[string]interface{} {
	article := articleViewFromRow(row)
	return map[string]interface{}{
		"articleId":     article.ArticleID.String(),
		"slug":          article.Slug,
		"canonicalSlug": article.Slug,
		"articleName":   article.ArticleName,
		"title":         article.Title,
		"summary":       article.Summary,
		"bodyMdx":       article.BodyMdx,
		"status":        string(article.Status),
		"version":       article.Version,
		"tags":          article.Tags,
		"featured":      article.Featured,
		"isFeatured":    article.Featured,
		"featuredRank":  article.FeaturedRank,
		"featuredAt":    timestamptzValue(article.FeaturedAt),
		"coverImageUrl": textValue(article.CoverImageUrl),
		"publishedAt":   timestamptzValue(article.PublishedAt),
		"authorId":      article.AuthorID.String(),
		"createdAt":     timestamptzValue(article.CreatedAt),
		"updatedAt":     timestamptzValue(article.UpdatedAt),
	}
}

func articleViewFromRow[T articleRow](row T) articleView {
	switch article := any(row).(type) {
	case dbgen.CreateDraftArticleRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.GetArticleByIDRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.GetArticleByArticleNameRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.GetArticleBySlugRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.GetArticleRevisionRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.ListAdminArticlesRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.ListPublishedArticlesRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.PublishArticleRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.MarkArticlePrivateRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.RestoreArticleToDraftRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.UnpublishArticleRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.UpdateDraftArticleRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case dbgen.UpdatePrivateArticleRow:
		return articleView{
			ArticleID:     article.ArticleID,
			Slug:          article.Slug,
			ArticleName:   article.ArticleName,
			Title:         article.Title,
			Summary:       article.Summary,
			BodyMdx:       article.BodyMdx,
			Status:        article.Status,
			Tags:          article.Tags,
			Featured:      article.Featured,
			FeaturedRank:  article.FeaturedRank,
			FeaturedAt:    article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl,
			PublishedAt:   article.PublishedAt,
			AuthorID:      article.AuthorID,
			CreatedAt:     article.CreatedAt,
			UpdatedAt:     article.UpdatedAt,
			Version:       article.Version,
		}
	case articleView:
		return article
	default:
		panic("unsupported article row type")
	}
}

func textValue(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}
