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

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}
