package articles

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestDeleteArticleDatabase(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	router := authenticatedArticleRouter(t, NewHandler(dbgen.New(pool)))
	exec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		testutil.Exec(t, pool, sql, args...)
	}
	exec(t, "insert into users(email, name, password_hash) values ('author@example.invalid','作者','unused')")
	exec(t, "insert into site_view_daily(view_date,views) values (current_date,5)")
	token := adminToken(t)
	remove := func(t *testing.T, id pgtype.UUID, version int, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/articles/%s?expectedVersion=%d", id.String(), version), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("delete status %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	for i, status := range []string{"draft", "published", "private", "archived"} {
		t.Run(status, func(t *testing.T) {
			var id pgtype.UUID
			err := pool.QueryRow(ctx, `
				insert into articles (slug,article_name,title,summary,body_mdx,status,author_id)
				select $1,$2,'文章','','正文',$3,user_id from users returning article_id
			`, fmt.Sprintf("art%03d", i), "article-"+status, status).Scan(&id)
			if err != nil {
				t.Fatal(err)
			}
			exec(t, "update articles set title='已修改' where article_id=$1", id)
			exec(t, "insert into comments(article_id,author_name,body,ip_hash) values ($1,'访客','评论','hash')", id)
			exec(t, "insert into article_view_daily(article_id,view_date,views) values ($1,current_date,1)", id)
			exec(t, "insert into article_view_history(article_id,view_date,views) values ($1,current_date-1,2)", id)
			exec(t, "insert into analytics_events(article_id,event_type,ip_hash) values ($1,'article_view','hash')", id)
			exec(t, "insert into postbridge_article_states(source_id,article_id,source_version,state_seq) values ('source',$1,2,1)", id)
			exec(t, `insert into postbridge_attempts (id,source_id,article_id,request_id,source_version,state_seq,target_account_id,
				authorization_id,authorization_expires_at,status,data)
				values ($2,'source',$1,'request',2,1,'account',$2,now()+interval '1 hour','prepared','{}')`, id, id.String())
			remove(t, id, 1, http.StatusConflict)
			remove(t, id, 2, http.StatusNoContent)
			remove(t, id, 2, http.StatusNotFound)
			for _, table := range []string{"articles", "article_logs", "comments", "article_view_daily", "article_view_history", "analytics_events"} {
				var n int
				if err := pool.QueryRow(ctx, "select count(*) from "+table+" where article_id=$1", id).Scan(&n); err != nil || n != 0 {
					t.Fatalf("%s residual count %d: %v", table, n, err)
				}
			}
			var revoked bool
			if err := pool.QueryRow(ctx, `select revoked_at is not null and cancel_pending from postbridge_attempts where article_id=$1`, id).Scan(&revoked); err != nil || !revoked {
				t.Fatal("分发授权未撤销", err)
			}
			var events int
			if err := pool.QueryRow(ctx, `select count(distinct event_type) from postbridge_events where article_id=$1 and event_type in ('authorization_revoked','deleted')`, id).Scan(&events); err != nil || events != 2 {
				t.Fatal("删除与撤销事件不完整", events, err)
			}
		})
	}
	var views int
	if err := pool.QueryRow(ctx, "select views from site_view_daily where view_date=current_date").Scan(&views); err != nil || views != 5 {
		t.Fatal("站点统计发生变化", views, err)
	}
}
