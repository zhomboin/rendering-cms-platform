package articles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestArticleTagMigration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{fmt.Sprintf("article_tags_test_%d", time.Now().UnixNano())}.Sanitize()
	t.Cleanup(func() {
		defer conn.Close(ctx)
		if conn.PgConn().TxStatus() != 'I' {
			_, _ = conn.Exec(ctx, "rollback")
		}
		if _, err := conn.Exec(ctx, "drop schema if exists "+schema+" cascade"); err != nil {
			t.Errorf("清理测试 schema: %v", err)
		}
	})
	exec := func(sql string) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exec("create schema " + schema)
	exec("set search_path to " + schema + ", public")

	const migration = "000017_normalize_article_tags"
	migrations := filepath.Join("..", "..", "migrations")
	apply := func(name string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join(migrations, name))
		if err != nil {
			t.Fatal(err)
		}
		exec(strings.TrimPrefix(string(sql), "create extension if not exists pgcrypto;"))
	}
	paths, err := filepath.Glob(filepath.Join(migrations, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if name := filepath.Base(path); name < migration+".up.sql" {
			apply(name)
		}
	}

	exec(`
		insert into users (email, name, password_hash)
		values ('tags@example.invalid', '测试用户', 'unused');
		insert into articles (slug, article_name, title, summary, body_mdx, status, tags, author_id)
		select 'abc001', 'tagged', '标签文章', '', '文章内容', 'published',
			array['OldTag', 'old_tag'], user_id from users;
		insert into articles (slug, article_name, title, summary, body_mdx, tags, author_id)
		select 'abc002', 'canonical', '规范标签', '', '文章内容', array['go'], user_id from users;
		update articles set tags = array['ReactDOM', ' react_dom ', ' Café__Tools ',
			'数据库 入门', 'Ｒｅｄｉｓ', 'नमस्ते दुनिया', null, '!!!', ''] where slug = 'abc001';
		insert into postbridge_article_states (source_id, article_id, source_version, state_seq)
		select 'test-source', article_id, version, nextval('postbridge_state_seq')
		from articles where slug = 'abc001';
	`)
	apply(migration + ".up.sql")

	assertTags := func(query string, want []string) {
		t.Helper()
		var got []string
		if err := conn.QueryRow(ctx, query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got == nil || !slices.Equal(got, want) {
			t.Fatalf("tags = %q, want %q (%s)", got, want, query)
		}
	}
	want := []string{"react-dom", "cafe-tools", "数据库-入门", "redis", "नमस्ते-दुनिया"}
	assertTags("select tags from articles where slug = 'abc001' and version = 3", want)
	assertTags("select tags from article_logs where slug = 'abc001' and version = 1", []string{"old-tag"})
	assertTags("select tags from article_logs where slug = 'abc001' and version = 2", want)
	assertTags("select tags from article_logs where slug = 'abc001' and version = 3", want)
	assertTags("select tags from articles where slug = 'abc002' and version = 1", []string{"go"})
	assertTags(`
		select array(select jsonb_array_elements_text(convert_from(snapshot, 'UTF8')::jsonb->'Tags'))
		from postbridge_events where event_type = 'updated'
		and convert_from(body, 'UTF8')::jsonb->>'source_version' = '3'
	`, want)

	assertTags(`
		insert into articles (slug, article_name, title, summary, body_mdx, tags, author_id)
		select 'abc003', 'new-tags', '新增标签', '', '文章内容', array['GoLang', 'go-lang', '---'], user_id
		from users returning tags
	`, []string{"go-lang"})
	assertTags("update articles set tags = null where slug = 'abc003' returning tags", []string{})
	assertTags("select tags from article_logs where slug = 'abc003' and version = 2", []string{})

	apply(migration + ".down.sql")
	assertTags("select tags from articles where slug = 'abc001' and version = 3", want)
	assertTags("update articles set tags = array['GoLang'] where slug = 'abc003' returning tags", []string{"GoLang"})
}
