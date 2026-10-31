package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

type dashboardStub struct {
	overview dbgen.GetDashboardOverviewRow
	recent   []dbgen.ListDashboardRecentArticlesRow
	comments []dbgen.ListDashboardPendingCommentsRow
	images   []dbgen.ListDashboardRecentImagesRow
	articles []dbgen.ListDashboardArticlesRow
	albums   []dbgen.ListDashboardAlbumsRow
	fail     string
	days     int32
}

func (s *dashboardStub) failure(name string) error {
	if s.fail == name {
		return errors.New("secret database details")
	}
	return nil
}
func (s *dashboardStub) GetDashboardOverview(ctx context.Context, days int32) (dbgen.GetDashboardOverviewRow, error) {
	s.days = days
	return s.overview, s.failure("overview")
}
func (s *dashboardStub) ListDashboardRecentArticles(ctx context.Context, arg dbgen.ListDashboardRecentArticlesParams) ([]dbgen.ListDashboardRecentArticlesRow, error) {
	name := "recent"
	if arg.Status == "draft" {
		name = "drafts"
	}
	return s.recent, s.failure(name)
}
func (s *dashboardStub) ListDashboardPendingComments(ctx context.Context) ([]dbgen.ListDashboardPendingCommentsRow, error) {
	return s.comments, s.failure("comments")
}
func (s *dashboardStub) ListDashboardRecentImages(ctx context.Context) ([]dbgen.ListDashboardRecentImagesRow, error) {
	return s.images, s.failure("images")
}
func (s *dashboardStub) ListDashboardArticles(ctx context.Context, days int32) ([]dbgen.ListDashboardArticlesRow, error) {
	s.days = days
	return s.articles, s.failure("articles")
}
func (s *dashboardStub) ListDashboardAlbums(ctx context.Context, days int32) ([]dbgen.ListDashboardAlbumsRow, error) {
	s.days = days
	return s.albums, s.failure("albums")
}

func requestDashboard(t *testing.T, s *dashboardStub, path, role string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(s)
	router := httpapi.NewRouter(httpapi.WithJWTSecret("dashboard-test-secret"), httpapi.WithAdminRoutes(h.RegisterAdminRoutes))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/"+path, nil)
	if role != "" {
		token, err := auth.IssueToken("dashboard-test-secret", "user", role)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
func TestDashboardDaysAndEmptyCollections(t *testing.T) {
	for _, endpoint := range []string{"overview", "articles", "albums"} {
		for _, tt := range []struct {
			raw  string
			want int32
		}{{"", 30}, {"7", 7}, {"30", 30}, {"90", 90}, {"0", 30}, {"-1", 30}, {"365", 30}, {"oops", 30}} {
			t.Run(endpoint+"/"+tt.raw, func(t *testing.T) {
				s := &dashboardStub{}
				rec := requestDashboard(t, s, endpoint+"?days="+tt.raw, "admin")
				if rec.Code != 200 {
					t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
				var body map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if s.days != tt.want || string(body["days"]) != stringNumber(tt.want) {
					t.Fatalf("days input=%q query=%d response=%s", tt.raw, s.days, body["days"])
				}
				keys := []string{endpoint}
				if endpoint == "overview" {
					keys = []string{"recentArticles", "drafts", "pendingComments", "recentImages"}
				}
				for _, key := range keys {
					if string(body[key]) != "[]" {
						t.Errorf("%s = %s, want []", key, body[key])
					}
				}
			})
		}
	}
}
func stringNumber(n int32) string {
	if n == 7 {
		return "7"
	}
	if n == 90 {
		return "90"
	}
	return "30"
}
func TestDashboardAuthorization(t *testing.T) {
	for _, endpoint := range []string{"overview", "articles", "albums"} {
		for _, tt := range []struct {
			role   string
			status int
		}{{"", 401}, {"viewer", 403}, {"admin", 200}, {"editor", 200}} {
			t.Run(endpoint+tt.role, func(t *testing.T) {
				rec := requestDashboard(t, &dashboardStub{}, endpoint, tt.role)
				if rec.Code != tt.status {
					t.Fatalf("status=%d want=%d", rec.Code, tt.status)
				}
			})
		}
	}
}
func TestDashboardQueryFailuresNeverReturnPartialSuccess(t *testing.T) {
	for _, failure := range []string{"overview", "recent", "drafts", "comments", "images", "articles", "albums"} {
		t.Run(failure, func(t *testing.T) {
			endpoint := "overview"
			if failure == "articles" || failure == "albums" {
				endpoint = failure
			}
			rec := requestDashboard(t, &dashboardStub{fail: failure}, endpoint, "admin")
			if rec.Code != 500 {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), "读取失败") {
				t.Fatalf("unsafe error: %s", rec.Body.String())
			}
		})
	}
}
func TestDashboardMapsSummariesWithoutSensitiveFields(t *testing.T) {
	now := pgtype.Timestamptz{Time: time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC), Valid: true}
	date := pgtype.Date{Time: now.Time, Valid: true}
	id := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	s := &dashboardStub{overview: dbgen.GetDashboardOverviewRow{GeneratedAt: now, StartDate: date, EndDate: date, PeriodViews: 5000000000, TotalBytes: 6000000000}}
	for i := 0; i < 8; i++ {
		s.recent = append(s.recent, dbgen.ListDashboardRecentArticlesRow{ArticleID: id, Slug: "abc123", Title: "文章", Status: dbgen.ArticleStatusDraft, UpdatedAt: now})
		s.comments = append(s.comments, dbgen.ListDashboardPendingCommentsRow{CommentID: id, ArticleID: id, ArticleTitle: "文章", Body: strings.Repeat("评", 160), CreatedAt: now})
		s.images = append(s.images, dbgen.ListDashboardRecentImagesRow{AssetID: id, Filename: "image.jpg", PublicUrl: "https://cdn.example/image.jpg", CreatedAt: now})
		s.articles = append(s.articles, dbgen.ListDashboardArticlesRow{ArticleID: id, Title: "文章", PeriodViews: 5000000000})
		s.albums = append(s.albums, dbgen.ListDashboardAlbumsRow{AlbumID: id, Title: "相册", CoverUrl: "https://cdn.example/cover.jpg", UpdatedAt: now})
	}
	rec := requestDashboard(t, s, "overview", "editor")
	if rec.Code != 200 {
		t.Fatalf("%s", rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]int{"recentArticles": 5, "drafts": 3, "pendingComments": 3, "recentImages": 6} {
		rows := body[key].([]interface{})
		if len(rows) != want {
			t.Errorf("%s length=%d want=%d", key, len(rows), want)
		}
	}
	if body["metrics"].(map[string]interface{})["periodViews"] != float64(5000000000) {
		t.Errorf("int64 count truncated: %s", rec.Body.String())
	}
	article := body["recentArticles"].([]interface{})[0].(map[string]interface{})
	if len(article["tags"].([]interface{})) != 0 || article["publishedAt"] != nil {
		t.Errorf("nullable summary=%v", article)
	}
	comment := body["pendingComments"].([]interface{})[0].(map[string]interface{})
	if len([]rune(comment["body"].(string))) > 121 {
		t.Errorf("comment preview too long")
	}
	for _, key := range []string{"authorEmail", "ipHash", "userAgent", "storageKey", "bodyMdx", "passwordHash"} {
		if strings.Contains(rec.Body.String(), key) {
			t.Errorf("sensitive field %s returned", key)
		}
	}
	for endpoint, want := range map[string]int{"articles": 5, "albums": 4} {
		rec = requestDashboard(t, s, endpoint, "admin")
		var response map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &response)
		if len(response[endpoint].([]interface{})) != want {
			t.Errorf("%s not bounded", endpoint)
		}
	}
}
func TestDashboardImagesOnlyReturnPublicHTTPURLs(t *testing.T) {
	s := &dashboardStub{}
	for _, u := range []string{"", "javascript:alert(1)", "/private/key", "https://user:pass@cdn.example/a", "http://", "https://cdn.example/a.jpg"} {
		s.images = append(s.images, dbgen.ListDashboardRecentImagesRow{PublicUrl: u})
	}
	rec := requestDashboard(t, s, "overview", "admin")
	var body struct {
		RecentImages []struct {
			PublicURL string `json:"publicUrl"`
		} `json:"recentImages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.RecentImages) != 1 || body.RecentImages[0].PublicURL != "https://cdn.example/a.jpg" {
		t.Fatalf("images=%s", rec.Body.String())
	}
}

func TestDashboardResponsesDisableCaching(t *testing.T) {
	for _, endpoint := range []string{"overview", "articles", "albums"} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/error=%t", endpoint, fail), func(t *testing.T) {
				stub := &dashboardStub{}
				if fail {
					stub.fail = endpoint
				}
				rec := requestDashboard(t, stub, endpoint, "admin")
				if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
					t.Fatalf("Cache-Control=%q", got)
				}
			})
		}
	}
}
