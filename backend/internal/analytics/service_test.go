package analytics

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestSummaryTotals(t *testing.T) {
	days := []DailyView{
		{Views: 10},
		{Views: 20},
		{Views: 7},
	}

	if total := TotalViews(days); total != 37 {
		t.Fatalf("TotalViews() = %d, want 37", total)
	}
}

func TestNormalizeArticleAnalyticsDays(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int32
	}{
		{name: "defaults to seven days", raw: "", want: 7},
		{name: "accepts valid value", raw: "30", want: 30},
		{name: "clamps value below range", raw: "0", want: 1},
		{name: "clamps value above range", raw: "120", want: 90},
		{name: "defaults invalid value", raw: "bad", want: 7},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeArticleAnalyticsDays(tt.raw); got != tt.want {
				t.Fatalf("normalizeArticleAnalyticsDays(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMapArticleAnalyticsRows(t *testing.T) {
	publishedAt := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
	body := mapArticleAnalyticsRows(7, []dbgen.ListArticleAnalyticsRowsRow{
		{
			Slug:        "mysql-mvcc-read-view-explained",
			Title:       "MySQL MVCC Read View Explained",
			TodayViews:  12,
			PeriodViews: 86,
			TotalViews:  324,
			PublishedAt: pgtype.Timestamptz{Time: publishedAt, Valid: true},
		},
	})

	if body["days"] != int32(7) {
		t.Fatalf("days = %#v, want 7", body["days"])
	}
	articles, ok := body["articles"].([]map[string]interface{})
	if !ok {
		t.Fatalf("articles has type %T, want []map[string]interface{}", body["articles"])
	}
	if len(articles) != 1 {
		t.Fatalf("len(articles) = %d, want 1", len(articles))
	}
	article := articles[0]
	for key, want := range map[string]interface{}{
		"slug":        "mysql-mvcc-read-view-explained",
		"title":       "MySQL MVCC Read View Explained",
		"todayViews":  int32(12),
		"periodViews": int32(86),
		"totalViews":  int32(324),
		"publishedAt": publishedAt,
	} {
		if article[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, article[key], want)
		}
	}
}
