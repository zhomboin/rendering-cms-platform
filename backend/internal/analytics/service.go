package analytics

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

type DailyView struct {
	Date  time.Time
	Views int
}

type DailyViewArchiver interface {
	ArchiveArticleViewsBeforeDate(ctx context.Context, cutoffDate pgtype.Date) error
	ArchiveSiteViewsBeforeDate(ctx context.Context, cutoffDate pgtype.Date) error
}

func ArchivePastDailyViews(ctx context.Context, archiver DailyViewArchiver, now time.Time) error {
	cutoff := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		Valid: true,
	}
	if err := archiver.ArchiveArticleViewsBeforeDate(ctx, cutoff); err != nil {
		return err
	}
	if err := archiver.ArchiveSiteViewsBeforeDate(ctx, cutoff); err != nil {
		return err
	}
	return nil
}

func TotalViews(days []DailyView) int {
	total := 0
	for _, day := range days {
		total += day.Views
	}
	return total
}

func normalizeArticleAnalyticsDays(raw string) int32 {
	if raw == "" {
		return 7
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 7
	}
	if days < 1 {
		return 1
	}
	if days > 90 {
		return 90
	}
	return int32(days)
}

func normalizeAnalyticsTrendDays(raw string) int32 {
	switch raw {
	case "7":
		return 7
	case "", "30":
		return 30
	case "90":
		return 90
	default:
		return 30
	}
}

func mapArticleAnalyticsRows(days int32, rows []dbgen.ListArticleAnalyticsRowsRow) map[string]interface{} {
	articles := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		articles = append(articles, map[string]interface{}{
			"slug":        row.Slug,
			"title":       row.Title,
			"todayViews":  row.TodayViews,
			"periodViews": row.PeriodViews,
			"totalViews":  row.TotalViews,
			"publishedAt": timestamptzValue(row.PublishedAt),
		})
	}
	return map[string]interface{}{
		"days":     days,
		"articles": articles,
	}
}

func mapAnalyticsTrend(days int32, siteRows []dbgen.ListSiteViewTrendRow, articleRows []dbgen.ListArticleViewTrendRow) map[string]interface{} {
	site := make([]map[string]interface{}, 0, len(siteRows))
	for _, row := range siteRows {
		site = append(site, map[string]interface{}{
			"date":  row.ViewDate.Time.Format("2006-01-02"),
			"views": row.Views,
		})
	}
	articles := make([]map[string]interface{}, 0, len(articleRows))
	for _, row := range articleRows {
		articles = append(articles, map[string]interface{}{
			"date":  row.ViewDate.Time.Format("2006-01-02"),
			"slug":  row.Slug,
			"title": row.Title,
			"views": row.Views,
		})
	}
	return map[string]interface{}{
		"days":     days,
		"site":     site,
		"articles": articles,
	}
}

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}
