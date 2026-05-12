package articles

import "rendering-cms-platform/backend/internal/database/dbgen"

func mapSearchResults(articles []dbgen.SearchPublishedArticlesRow) []map[string]interface{} {
	response := make([]map[string]interface{}, 0, len(articles))
	for _, article := range articles {
		response = append(response, map[string]interface{}{
			"articleId":   article.ArticleID.String(),
			"slug":        article.Slug,
			"title":       article.Title,
			"summary":     article.Summary,
			"publishedAt": timestamptzValue(article.PublishedAt),
		})
	}
	return response
}
