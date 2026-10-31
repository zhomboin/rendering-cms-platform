package dashboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

// TEST_DATABASE_URL 必须显式指向可创建隔离 schema 的测试数据库。
func dashboardTestDatabase(t *testing.T) (*pgx.Conn, *dbgen.Queries) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("未设置 TEST_DATABASE_URL，跳过 PostgreSQL 集成测试")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "create extension if not exists pgcrypto with schema public"); err != nil {
		conn.Close(ctx)
		t.Fatal(err)
	}
	schema := fmt.Sprintf("dashboard_test_%d", time.Now().UnixNano())
	name := pgx.Identifier{schema}.Sanitize()
	t.Cleanup(func() {
		defer conn.Close(context.Background())
		if _, err := conn.Exec(context.Background(), "drop schema if exists "+name+" cascade"); err != nil {
			t.Errorf("清理测试 schema: %v", err)
		}
	})
	if _, err := conn.Exec(ctx, "create schema "+name); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "set search_path to "+name+", public"); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("读取迁移: paths=%v error=%v", paths, err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("迁移 %s: %v", filepath.Base(path), err)
		}
	}
	return conn, dbgen.New(conn)
}
func execDashboardFixture(t *testing.T, conn *pgx.Conn, sql string) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), sql); err != nil {
		t.Fatalf("测试数据写入失败: %v", err)
	}
}
func TestDashboardSQLIntegration(t *testing.T) {
	conn, q := dashboardTestDatabase(t)
	ctx := context.Background()
	t.Run("空数据库", func(t *testing.T) {
		r, err := q.GetDashboardOverview(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if r.TodayViews != 0 || r.PeriodViews != 0 || r.AssetCount != 0 || r.PublishedArticles != 0 {
			t.Fatalf("空聚合=%+v", r)
		}
		for _, days := range []int32{7, 30, 90} {
			r, err = q.GetDashboardOverview(ctx, days)
			if err != nil {
				t.Fatal(err)
			}
			if r.EndDate.Time.Sub(r.StartDate.Time) != time.Duration(days-1)*24*time.Hour || r.GeneratedAt.Time.IsZero() {
				t.Fatalf("窗口=%+v", r)
			}
		}
	})
	execDashboardFixture(t, conn, `
 insert into users(user_id,email,name,password_hash) values ('00000000-0000-0000-0000-000000000001','test@example.invalid','测试用户','unused');
 insert into articles(article_id,slug,article_name,title,summary,body_mdx,status,published_at,author_id,updated_at)
 select ('10000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,lpad(n::text,6,'0'),'test-article-'||n,'文章'||n,'摘要','不应进入摘要的正文',
 (case when n<=6 then 'published' when n=7 then 'draft' when n=8 then 'private' else 'archived' end)::article_status,
 case when n<=6 then now()-make_interval(days=>n) end,'00000000-0000-0000-0000-000000000001',now()-make_interval(days=>n)
 from generate_series(1,9) n;
 insert into site_view_daily values (current_date,3),(current_date-1,2),(current_date-6,5),(current_date-7,10),(current_date+1,999);
 insert into site_view_history(view_date,views) values (current_date-1,7),(current_date-6,11),(current_date-7,13),(current_date,999),(current_date+1,999);
 insert into article_view_daily(article_id,view_date,views) values
 ('10000000-0000-0000-0000-000000000001',current_date,1600000000),
 ('10000000-0000-0000-0000-000000000001',current_date-1,1600000000),
 ('10000000-0000-0000-0000-000000000001',current_date-6,6),
 ('10000000-0000-0000-0000-000000000001',current_date-7,9),
 ('10000000-0000-0000-0000-000000000001',current_date+1,9000),
 ('10000000-0000-0000-0000-000000000002',current_date-1,4),
 ('10000000-0000-0000-0000-000000000007',current_date,90),
 ('10000000-0000-0000-0000-000000000008',current_date,90),
 ('10000000-0000-0000-0000-000000000009',current_date,90);
 insert into article_view_history(article_id,view_date,views) values
 ('10000000-0000-0000-0000-000000000001',current_date-2,1600000000),
 ('10000000-0000-0000-0000-000000000001',current_date-6,7),
 ('10000000-0000-0000-0000-000000000001',current_date-7,8),
 ('10000000-0000-0000-0000-000000000001',current_date,999),
 ('10000000-0000-0000-0000-000000000001',current_date+1,999),
 ('10000000-0000-0000-0000-000000000002',current_date-6,5);
 insert into photo_albums(album_id,slug,title,description,status,created_by,updated_at)
 select ('20000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,'album-'||n,'相册'||n,'说明',
 (case when n<=2 then 'published' else 'draft' end)::photo_album_status,'00000000-0000-0000-0000-000000000001',now()-make_interval(days=>n)
 from generate_series(1,5)n;
 insert into album_view_daily values
 ('20000000-0000-0000-0000-000000000001',current_date,2),
 ('20000000-0000-0000-0000-000000000001',current_date-6,3),
 ('20000000-0000-0000-0000-000000000001',current_date-7,20),
 ('20000000-0000-0000-0000-000000000001',current_date+1,99),
 ('20000000-0000-0000-0000-000000000003',current_date,7);
 insert into assets(asset_id,filename,content_type,byte_size,storage_key,public_url,status,deleted_at,created_by)
 select ('30000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,'file-'||n,
 case when n=3 then 'application/pdf' else 'image/jpeg' end,
 case when n<=3 then 1700000000 when n=4 then 10 when n=8 then 20 else 100 end,
 'private-object-key-'||n,
 case when n=4 then null when n=8 then 'javascript:alert(1)' else 'https://cdn.example/file-'||n||'.jpg' end,
 (case when n=5 then 'archived' when n=6 then 'deleted' else 'active' end)::asset_status,
 case when n in(6,7) then now() end,'00000000-0000-0000-0000-000000000001'
 from generate_series(1,8)n;
 insert into album_photos(photo_id,album_id,asset_id,alt,width,height,position) values
 ('40000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000006','删除封面',1,1,0),
 ('40000000-0000-0000-0000-000000000002','20000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000001','回退封面',1,1,1),
 ('40000000-0000-0000-0000-000000000003','20000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000001','第一张',1,1,0),
 ('40000000-0000-0000-0000-000000000004','20000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000002','明确封面',1,1,1),
 ('40000000-0000-0000-0000-000000000005','20000000-0000-0000-0000-000000000003','30000000-0000-0000-0000-000000000005','归档照片',1,1,0),
 ('40000000-0000-0000-0000-000000000006','20000000-0000-0000-0000-000000000003','30000000-0000-0000-0000-000000000008','非法链接',1,1,1);
 update photo_albums set cover_photo_id='40000000-0000-0000-0000-000000000001' where album_id='20000000-0000-0000-0000-000000000001';
 update photo_albums set cover_photo_id='40000000-0000-0000-0000-000000000004' where album_id='20000000-0000-0000-0000-000000000002';
 insert into comments(article_id,author_name,author_email,body,status,ip_hash,user_agent,created_at)
 select '10000000-0000-0000-0000-000000000001','读者','private@example.invalid',repeat('评',160),'pending','sensitive-hash','sensitive-agent',current_date+interval '1 hour' from generate_series(1,5);
 insert into comments(article_id,author_name,body,status,ip_hash,created_at) values
 ('10000000-0000-0000-0000-000000000001','边界','内容','approved','hash',current_date-6),
 ('10000000-0000-0000-0000-000000000001','过期','内容','approved','hash',current_date-7),
 ('10000000-0000-0000-0000-000000000001','未来','内容','rejected','hash',current_date+1);
 `)
	t.Run("日期归档与有效资源聚合", func(t *testing.T) {
		r, err := q.GetDashboardOverview(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if r.TodayViews != 3 || r.PeriodViews != 28 || r.ArticleViews != 4800000022 || r.AlbumViews != 12 {
			t.Fatalf("访问聚合=%+v", r)
		}
		if r.NewComments != 6 || r.PendingComments != 5 || r.PublishedArticles != 6 || r.DraftArticles != 1 || r.PrivateArticles != 1 || r.ArchivedArticles != 1 || r.PublishedAlbums != 2 || r.DraftAlbums != 3 || r.PhotoCount != 4 || r.MissingCoverArticles != 6 {
			t.Fatalf("内容聚合=%+v", r)
		}
		if r.AssetCount != 5 || r.ImageCount != 4 || r.TotalBytes != 5100000030 {
			t.Fatalf("资源聚合=%+v", r)
		}
	})
	t.Run("文章排序上限零量及私有隔离", func(t *testing.T) {
		rows, err := q.ListDashboardArticles(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 5 || rows[0].Slug != "000001" || rows[1].Slug != "000002" || rows[2].PeriodViews != 0 || rows[0].PeriodViews != 4800000013 || rows[0].TotalViews != 4800000030 {
			t.Fatalf("文章排名=%+v", rows)
		}
		for _, r := range rows {
			if r.Slug >= "000007" {
				t.Fatalf("非公开文章进入排名: %+v", r)
			}
		}
		recent, err := q.ListDashboardRecentArticles(ctx, dbgen.ListDashboardRecentArticlesParams{Status: "", RowLimit: 5})
		if err != nil || len(recent) != 5 {
			t.Fatalf("最近文章=%+v %v", recent, err)
		}
		drafts, err := q.ListDashboardRecentArticles(ctx, dbgen.ListDashboardRecentArticlesParams{Status: "draft", RowLimit: 3})
		if err != nil || len(drafts) != 1 || drafts[0].Slug != "000007" {
			t.Fatalf("草稿隔离=%+v %v", drafts, err)
		}
	})
	t.Run("相册发布优先及封面回退", func(t *testing.T) {
		rows, err := q.ListDashboardAlbums(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 4 || rows[0].Slug != "album-1" || rows[1].Slug != "album-2" || rows[2].Slug != "album-3" || rows[2].PeriodViews != 7 {
			t.Fatalf("相册排序=%+v", rows)
		}
		if rows[0].CoverUrl != "https://cdn.example/file-1.jpg" || rows[1].CoverUrl != "https://cdn.example/file-2.jpg" || rows[2].CoverUrl != "" || rows[0].PhotoCount != 1 {
			t.Fatalf("相册封面=%+v", rows)
		}
	})
	t.Run("评论与图片有限摘要", func(t *testing.T) {
		comments, err := q.ListDashboardPendingComments(ctx)
		if err != nil || len(comments) != 3 {
			t.Fatalf("待审核=%+v %v", comments, err)
		}
		if len([]rune(comments[0].Body)) != 120 {
			t.Fatalf("正文未在数据库截断: %d", len([]rune(comments[0].Body)))
		}
		images, err := q.ListDashboardRecentImages(ctx)
		if err != nil || len(images) != 2 {
			t.Fatalf("公开有效图片=%+v %v", images, err)
		}
		for _, r := range images {
			if !strings.HasPrefix(r.PublicUrl, "https://cdn.example/") {
				t.Errorf("非法图片链接=%s", r.PublicUrl)
			}
		}
	})
	t.Run("长窗口仍排除未来", func(t *testing.T) {
		for _, days := range []int32{30, 90} {
			r, err := q.GetDashboardOverview(ctx, days)
			if err != nil {
				t.Fatal(err)
			}
			if r.PeriodViews != 51 || r.ArticleViews != 4800000039 || r.AlbumViews != 32 {
				t.Fatalf("%d天聚合=%+v", days, r)
			}
		}
	})
	t.Run("草稿与图片数据库上限", func(t *testing.T) {
		execDashboardFixture(t, conn, `
   insert into articles(article_id,slug,article_name,title,summary,body_mdx,status,author_id)
   select ('10000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,lpad(n::text,6,'0'),'extra-draft-'||n,'草稿','摘要','正文','draft','00000000-0000-0000-0000-000000000001' from generate_series(10,14)n;
   insert into assets(filename,content_type,byte_size,storage_key,public_url,created_by)
   select 'extra-image-'||n,'image/webp',1,'extra-key-'||n,'https://cdn.example/extra-'||n||'.webp','00000000-0000-0000-0000-000000000001' from generate_series(1,8)n;
  `)
		drafts, err := q.ListDashboardRecentArticles(ctx, dbgen.ListDashboardRecentArticlesParams{Status: "draft", RowLimit: 3})
		if err != nil || len(drafts) != 3 {
			t.Fatalf("草稿数据库上限=%d %v", len(drafts), err)
		}
		images, err := q.ListDashboardRecentImages(ctx)
		if err != nil || len(images) != 6 {
			t.Fatalf("图片数据库上限=%d %v", len(images), err)
		}
	})
	t.Run("归档后聚合不变", func(t *testing.T) {
		before, err := q.GetDashboardOverview(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		today := before.EndDate
		if err := q.ArchiveArticleViewsBeforeDate(ctx, today); err != nil {
			t.Fatal(err)
		}
		if err := q.ArchiveSiteViewsBeforeDate(ctx, today); err != nil {
			t.Fatal(err)
		}
		after, err := q.GetDashboardOverview(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if before.PeriodViews != after.PeriodViews || before.ArticleViews != after.ArticleViews {
			t.Fatalf("归档重复或丢量: before=%+v after=%+v", before, after)
		}
	})
}
