package articlemcp

import (
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestPermissionMigrationPreservesGrants(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	var owner pgtype.UUID
	if err := pool.QueryRow(ctx, "insert into users(email,name,password_hash) values ('permissions@example.invalid','权限测试','unused') returning user_id").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	apply := func(direction string) {
		t.Helper()
		sql, err := os.ReadFile("../../migrations/000021_mcp_permissions." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		testutil.Exec(t, pool, string(sql))
	}
	q := dbgen.New(pool)
	_, err := q.CreateMCPConnection(ctx, dbgen.CreateMCPConnectionParams{OwnerID: owner, Label: "受限", TokenHash: []byte{1}, DraftPermissions: []string{statusTool}, AllowPublish: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/000022_mcp_albums.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Exec(t, pool, string(raw))
	apply("down")
	testutil.Exec(t, pool, "insert into article_mcp_connections(owner_id,label,token_hash,allow_publish) values ($1,'草稿',$2,false),($1,'发布',$3,true)", owner, []byte{2}, []byte{3})
	apply("up")
	rows, err := q.ListMCPConnections(ctx, owner)
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Revoked != (row.Label == "受限") || row.AllowPublish != (row.Label != "草稿") || !slices.Equal(row.DraftPermissions, draftTools) {
			t.Fatalf("迁移往返改变授权：%s", row.Label)
		}
	}
}
