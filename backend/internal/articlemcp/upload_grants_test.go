package articlemcp

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func checkUploadGrantAPI(t *testing.T, pool *pgxpool.Pool, owner pgtype.UUID, request func(string, string, string, string, int) []byte, admin, anotherAdmin string) {
	const base = "/api/v1/admin/mcp/connections"
	ctx := t.Context()
	album, err := dbgen.New(pool).CreatePhotoAlbum(ctx, dbgen.CreatePhotoAlbumParams{CreatedBy: owner, Slug: "append", Title: "追加测试", Description: "授权", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	target := album.AlbumID.String()
	create := fmt.Sprintf(`{"label":"追加","permissions":["upload_album_photo"],"publishedUploadAlbumIds":[%q]}`, target)
	request("POST", base, admin, create, 400)
	testutil.Exec(t, pool, "update photo_albums set status='published',published_at=now() where album_id=$1", album.AlbumID)
	var created grantInfo
	if err = json.Unmarshal(request("POST", base, admin, create, 201), &created); err != nil || !slices.Equal(created.PublishedUploadAlbumIDs, []string{target}) {
		t.Fatal(created, err)
	}
	path := base + "/" + created.ID
	check := func(revision int32, targets ...string) {
		t.Helper()
		var rows []grantInfo
		if err := json.Unmarshal(request("GET", base, admin, "", 200), &rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.ID == created.ID {
				if row.PermissionRevision != revision || !slices.Equal(row.PublishedUploadAlbumIDs, targets) {
					t.Fatal(row)
				}
				return
			}
		}
		t.Fatal("缺少连接")
	}
	request("PATCH", path, anotherAdmin, `{"publishedUploadAlbumIds":[],"expectedRevision":1}`, 409)
	request("PATCH", path, admin, `{"publishedUploadAlbumIds":null}`, 400)
	request("PATCH", path, admin, fmt.Sprintf(`{"publishedUploadAlbumIds":[%q,%q]}`, target, target), 400)
	request("PATCH", path, admin, `{"permissions":[],"publishedUploadAlbumIds":["00000000-0000-0000-0000-000000000002"]}`, 400)
	request("PATCH", path, admin, `{"publishedUploadAlbumIds":["00000000-0000-0000-0000-000000000002"]}`, 400)
	check(1, target)
	request("PATCH", path, admin, `{"readScope":"all","expectedRevision":1}`, 204)
	check(2, target)
	request("PATCH", path, admin, `{"publishedUploadAlbumIds":[],"expectedRevision":1}`, 409)
	check(2, target)
	request("PATCH", path, admin, `{"publishedUploadAlbumIds":[],"expectedRevision":2}`, 204)
	check(3)
	request("PATCH", path, admin, fmt.Sprintf(`{"publishedUploadAlbumIds":[%q],"expectedRevision":3}`, target), 204)
	testutil.Exec(t, pool, "update photo_albums set status='draft' where album_id=$1", album.AlbumID)
	request("PATCH", path, admin, fmt.Sprintf(`{"permissions":["upload_album_photo"],"publishedUploadAlbumIds":[%q]}`, target), 204)
	check(5, target)
	request("PATCH", path, admin, `{"permissions":[]}`, 204)
	request("PATCH", path, admin, `{"permissions":["upload_album_photo"]}`, 204)
	check(7)
}
