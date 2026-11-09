package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func analyticsDatabaseFixture(t *testing.T) (*pgxpool.Pool, chi.Router) {
	t.Helper()
	pool := testutil.Database(t)
	testutil.Exec(t, pool, "insert into users(user_id,email,name,password_hash) values('11111111-1111-1111-1111-111111111111','fixture@example.invalid','测试','synthetic')")
	testutil.Exec(t, pool, "insert into articles(slug,article_name,title,summary,body_mdx,status,author_id) values('aB3dE9','docker','测试文章','','正文','published','11111111-1111-1111-1111-111111111111')")
	testutil.Exec(t, pool, "insert into photo_albums(slug,title,description,status,created_by) values('published-album','测试','描述','published','11111111-1111-1111-1111-111111111111'),('draft-album','草稿','描述','draft','11111111-1111-1111-1111-111111111111')")
	router := chi.NewRouter()
	NewHandler(dbgen.New(pool)).RegisterPublicRoutes(router)
	return pool, router
}

func databaseCount(t *testing.T, pool *pgxpool.Pool, query string) int {
	t.Helper()
	var result int
	if err := pool.QueryRow(context.Background(), query).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAnalyticsAllAggregatesRollbackAndRetryWithoutLosingCount(t *testing.T) {
	for _, route := range []struct{ path, table string }{
		{"/api/v1/articles/docker/views", "article_view_daily"},
		{"/api/v1/photos/published-album/views", "album_view_daily"},
		{"/api/v1/analytics/site-views", "site_view_daily"},
	} {
		t.Run(route.table, func(t *testing.T) {
			pool, router := analyticsDatabaseFixture(t)
			testutil.Exec(t, pool, "create function fail_site_counter() returns trigger language plpgsql as $$ begin raise exception 'injected count failure'; end $$")
			testutil.Exec(t, pool, "create trigger fail_site_counter before insert or update on site_view_daily for each row execute function fail_site_counter()")
			send := func(want int) {
				t.Helper()
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, route.path, nil))
				if response.Code != want {
					t.Fatalf("status=%d want=%d body=%s", response.Code, want, response.Body.String())
				}
			}
			send(500)
			if databaseCount(t, pool, "select count(*) from analytics_events") != 0 ||
				databaseCount(t, pool, "select count(*) from "+route.table) != 0 {
				t.Fatal("event or aggregate survived a failed transaction")
			}
			testutil.Exec(t, pool, "drop trigger fail_site_counter on site_view_daily")
			send(204)
			send(204)
			if databaseCount(t, pool, "select count(*) from analytics_events") != 1 ||
				databaseCount(t, pool, "select coalesce(sum(views),0) from "+route.table) != 1 ||
				databaseCount(t, pool, "select coalesce(sum(views),0) from site_view_daily") != 1 {
				t.Fatal("retry or duplicate request lost or duplicated a daily count")
			}
			if databaseCount(t, pool, "select count(*) from analytics_events e join "+route.table+" d on d.view_date = e.event_date") != 1 {
				t.Fatal("event and daily aggregate used different dates")
			}
		})
	}
}

func TestAlbumViewsRejectMissingAndUnpublishedAndDeduplicate(t *testing.T) {
	pool, router := analyticsDatabaseFixture(t)
	for _, test := range []struct {
		slug   string
		status int
	}{
		{"missing", 404}, {"draft-album", 404}, {"published-album", 204}, {"published-album", 204},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/photos/"+test.slug+"/views", nil))
		if response.Code != test.status {
			t.Fatalf("slug=%s status=%d want=%d", test.slug, response.Code, test.status)
		}
	}
	if databaseCount(t, pool, "select count(*) from analytics_events") != 1 ||
		databaseCount(t, pool, "select sum(views) from album_view_daily") != 1 ||
		databaseCount(t, pool, "select sum(views) from site_view_daily") != 1 {
		t.Fatal("album event was not counted exactly once with site count")
	}
	testutil.Exec(t, pool, "update photo_albums set status='draft' where slug='published-album'")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/photos/published-album/views", nil))
	if response.Code != 404 {
		t.Fatal("unpublished album accepted a view")
	}
}
