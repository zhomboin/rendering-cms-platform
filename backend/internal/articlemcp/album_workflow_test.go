package articlemcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/albums"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
	"rendering-cms-platform/backend/internal/testutil"
)

type photoObjects struct {
	puts, deletes int
	afterPut      func()
}

func (f *photoObjects) PutObject(_ context.Context, _ string, _ []byte, _ string, _ string) error {
	f.puts++
	if f.afterPut != nil {
		f.afterPut()
	}
	return nil
}
func (f *photoObjects) DeleteObject(context.Context, string) error { f.deletes++; return nil }

func TestAlbumToolsThroughMCP(t *testing.T) {
	pool := testutil.Database(t)
	ctx := t.Context()
	var owner pgtype.UUID
	if err := pool.QueryRow(ctx, "insert into users(email,name,password_hash) values ('photos@example.invalid','测试','unused') returning user_id").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(pool)
	objects := &photoObjects{}
	h := albums.NewHandlerWithOptions(albums.NewDatabaseStore(pool), objects, albums.HandlerOptions{PublicBaseURL: "https://objects.example.invalid"})
	s := New(pool, config.ArticleMCPConfig{Origin: "https://cms.example.invalid"}).WithAlbums(h)
	host := httptest.NewServer(httpapi.NewRouter(httpapi.WithPublicRoutes(s.Register)))
	t.Cleanup(host.Close)
	connect := func(label string, permissions []string) (*mcp.ClientSession, pgtype.UUID, string) {
		t.Helper()
		token := tokenPrefix + label
		hash := sha256.Sum256([]byte(token))
		id, err := q.CreateMCPConnection(ctx, dbgen.CreateMCPConnectionParams{OwnerID: owner, Label: label, TokenHash: hash[:], DraftPermissions: permissions})
		if err != nil {
			t.Fatal(err)
		}
		client := mcp.NewClient(&mcp.Implementation{Name: "album-test", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: host.URL + endpoint, HTTPClient: &http.Client{Transport: tokenTransport(token)}, DisableStandaloneSSE: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { session.Close() })
		return session, id, token
	}
	session, id, token := connect("albums", albumTools)
	other, otherID, _ := connect("other", albumTools)
	call := func(client *mcp.ClientSession, name string, in any, fail bool, out any) {
		t.Helper()
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: in})
		if err != nil {
			if fail {
				return
			}
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError != fail {
			t.Fatalf("%s error=%v: %v", name, res.IsError, res.Content)
		}
		if !fail && out != nil {
			raw, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, out); err != nil {
				t.Fatal(err)
			}
		}
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != len(albumTools) {
		t.Fatal(tools, err)
	}
	for _, tool := range tools.Tools {
		if !slices.Contains(albumTools, tool.Name) {
			t.Fatal(tool.Name)
		}
	}
	create := AlbumCreate{RequestKey: "one", AlbumFields: AlbumFields{Slug: "gallery", Title: "测试", Description: "草稿"}}
	var album, again AlbumRef
	call(session, createAlbumTool, create, false, &album)
	call(session, createAlbumTool, create, false, &again)
	if album.AlbumID == "" || again != album {
		t.Fatal(album, again)
	}
	create.Title = "冲突"
	call(session, createAlbumTool, create, true, nil)
	var listing AlbumList
	call(other, listAlbumsTool, AlbumPage{}, false, &listing)
	if len(listing.Items) != 0 {
		t.Fatal("跨连接枚举相册")
	}
	call(session, listAlbumsTool, AlbumPage{}, false, &listing)
	if len(listing.Items) != 1 || listing.Items[0].AlbumRef != album {
		t.Fatal(listing)
	}
	call(other, getAlbumTool, album, true, nil)
	call(other, uploadPhotoTool, PhotoUploadInput{AlbumRef: album, RequestKey: "x"}, true, nil)
	call(session, publishAlbumTool, album, true, nil)
	call(session, updateAlbumTool, AlbumEdit{AlbumRef: album, AlbumFields: create.AlbumFields}, false, nil)
	var details AlbumDetails
	call(session, getAlbumTool, album, false, &details)
	if details.Title != create.Title {
		t.Fatal(details)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	upload := PhotoUploadInput{AlbumRef: album, RequestKey: "first", Alt: "照片", Filename: "one.png", DataBase64: base64.StdEncoding.EncodeToString(data.Bytes())}
	var a, b UploadResult
	call(session, uploadPhotoTool, upload, false, &a)
	call(session, uploadPhotoTool, upload, false, &b)
	if a.PhotoID == "" || a.PhotoID != b.PhotoID || objects.puts != 1 {
		t.Fatal(a, b, objects.puts)
	}
	upload.Alt = "变更"
	call(session, uploadPhotoTool, upload, true, nil)
	var dest UploadResult
	call(session, uploadPhotoTool, PhotoUploadInput{AlbumRef: album, RequestKey: "second"}, false, &dest)
	if !strings.HasSuffix(dest.UploadURL, "/"+album.AlbumID+"/photos") {
		t.Fatal(dest)
	}
	sendFile := func(key string, want int) UploadResult {
		t.Helper()
		var buf bytes.Buffer
		form := multipart.NewWriter(&buf)
		part, err := form.CreateFormFile("file", "two.png")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(data.Bytes())
		form.WriteField("alt", "第二张")
		form.Close()
		req, err := http.NewRequest("POST", host.URL+strings.TrimPrefix(dest.UploadURL, s.cfg.Origin), &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", form.FormDataContentType())
		resp, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("上传 %d %s", resp.StatusCode, raw)
		}
		var result UploadResult
		if want == 201 && json.Unmarshal(raw, &result) != nil {
			t.Fatal("上传回执无效")
		}
		return result
	}
	// 在 IEND 前插入带正确 CRC 的可剥离文本块，覆盖恰好 50 MiB 的有效 PNG。
	small := slices.Clone(data.Bytes())
	data.Reset()
	data.Write(small[:len(small)-12])
	metadata := make([]byte, (50<<20)-len(small)-12)
	copy(metadata, "comment\x00")
	binary.Write(&data, binary.BigEndian, uint32(len(metadata)))
	start := data.Len()
	data.WriteString("tEXt")
	data.Write(metadata)
	binary.Write(&data, binary.BigEndian, crc32.ChecksumIEEE(data.Bytes()[start:]))
	data.Write(small[len(small)-12:])
	b = sendFile("second", 201)
	if again := sendFile("second", 201); again.PhotoID != b.PhotoID {
		t.Fatal("文件上传重试未返回原照片")
	}
	if b.PhotoID == a.PhotoID || b.PhotoID == "" {
		t.Fatal(b)
	}
	data.Reset()
	data.Write(small)
	first := PhotoRef{AlbumRef: album, PhotoID: a.PhotoID}
	manual, err := q.CreatePhotoAlbum(ctx, dbgen.CreatePhotoAlbumParams{CreatedBy: owner, Slug: "manual", Title: "后台相册", Description: "后台创建", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	setScope := func(all bool) {
		t.Helper()
		if _, err := q.ChangeMCPPermissions(ctx, dbgen.ChangeMCPPermissionsParams{OwnerID: owner, ID: otherID, ReadAll: pgtype.Bool{Bool: all, Valid: true}}); err != nil {
			t.Fatal(err)
		}
	}
	setScope(true)
	call(other, listAlbumsTool, AlbumPage{}, false, &listing)
	if len(listing.Items) != 2 {
		t.Fatal("应同时看到后台和其他连接的相册", listing)
	}
	call(other, getAlbumTool, AlbumRef{manual.AlbumID.String()}, false, nil)
	var sharedPhotos PhotoList
	call(other, listPhotosTool, PhotosPage{AlbumRef: album}, false, &sharedPhotos)
	if len(sharedPhotos.Items) != 2 {
		t.Fatal("未读到相册照片")
	}
	call(other, updateAlbumTool, AlbumEdit{AlbumRef: album, AlbumFields: create.AlbumFields}, true, nil)
	call(other, uploadPhotoTool, PhotoUploadInput{AlbumRef: album, RequestKey: "foreign"}, true, nil)
	call(other, deletePhotoTool, first, true, nil)
	call(other, publishAlbumTool, album, true, nil)
	setScope(false)
	call(other, listAlbumsTool, AlbumPage{}, false, &listing)
	if len(listing.Items) != 0 {
		t.Fatal("收窄范围后仍可列出相册")
	}
	call(other, listPhotosTool, PhotosPage{AlbumRef: album}, true, nil)
	call(other, getAlbumTool, AlbumRef{manual.AlbumID.String()}, true, nil)
	call(session, updatePhotoTool, PhotoEdit{PhotoRef: first, Alt: "说明", Caption: "描述"}, false, nil)
	call(session, orderPhotosTool, PhotoOrder{AlbumRef: album, PhotoIDs: []string{a.PhotoID, a.PhotoID}}, true, nil)
	call(session, orderPhotosTool, PhotoOrder{AlbumRef: album, PhotoIDs: []string{b.PhotoID, a.PhotoID}}, false, nil)
	call(session, coverTool, AlbumCover{AlbumRef: album, PhotoID: a.PhotoID}, false, nil)
	var photos PhotoList
	call(session, listPhotosTool, PhotosPage{AlbumRef: album, AlbumPage: AlbumPage{Offset: 1, Limit: 1}}, false, &photos)
	if len(photos.Items) != 1 || photos.Items[0].Alt != "说明" || photos.Items[0].PhotoID != a.PhotoID {
		t.Fatal(photos)
	}
	call(session, publishAlbumTool, album, false, nil)
	call(session, deletePhotoTool, first, true, nil)
	call(session, unpublishAlbumTool, album, false, nil)
	call(session, deletePhotoTool, first, false, nil)
	call(session, getAlbumTool, album, false, &details)
	if details.PhotoCount != 1 || details.CoverPhotoID != "" {
		t.Fatal(details)
	}
	set := func(tools []string) {
		t.Helper()
		_, err := q.ChangeMCPPermissions(ctx, dbgen.ChangeMCPPermissionsParams{ID: id, OwnerID: owner, DraftPermissions: tools})
		if err != nil {
			t.Fatal(err)
		}
	}
	// 在对象已写入且照片尚未关联时撤权，事务须拒绝关联并清理暂存对象。
	objects.afterPut = func() { set([]string{getAlbumTool}) }
	sendFile("in-flight", 403)
	if objects.deletes != 2 {
		t.Fatal("未清理删除照片或被撤权的上传", objects.deletes)
	}
	sendFile("revoked", 403)
	tools, err = session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != getAlbumTool {
		t.Fatal(tools, err)
	}
	call(session, listPhotosTool, PhotosPage{AlbumRef: album}, true, nil)
	call(session, getAlbumTool, album, false, &details)
	if details.PhotoCount != 1 {
		t.Fatal(details)
	}
	history, err := q.ListMCPOperations(ctx, dbgen.ListMCPOperationsParams{OwnerID: owner})
	if err != nil {
		t.Fatal(err)
	}
	var success, failed bool
	for _, op := range history {
		if op.Operation == uploadPhotoTool && op.Outcome == "success" && op.AlbumID.String() == album.AlbumID {
			success = true
		}
		if op.Operation == uploadPhotoTool && op.Outcome == "failed" {
			failed = true
		}
	}
	if !success || !failed {
		t.Fatal("相册上传审计不完整")
	}
	set([]string{readTool, uploadPhotoTool})
	for _, direction := range []string{"down", "up"} {
		raw, err := os.ReadFile("../../migrations/000022_mcp_albums." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		testutil.Exec(t, pool, string(raw))
		grant, err := q.LockMCPConnection(ctx, id)
		if err != nil || !slices.Equal(grant.DraftPermissions, []string{readTool}) {
			t.Fatal("迁移改变文章授权", grant, err)
		}
		row, err := q.GetPhotoAlbumByID(ctx, mustArticleID(t, album.AlbumID))
		if err != nil || row.PhotoCount != 1 {
			t.Fatal("迁移丢失相册或照片", row.PhotoCount, err)
		}
	}
}
