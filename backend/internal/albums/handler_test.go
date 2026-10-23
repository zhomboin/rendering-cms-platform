package albums

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

func uuidForTest(value string) pgtype.UUID {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		panic(err)
	}
	return id
}

const (
	testAlbumID = "11111111-1111-1111-1111-111111111111"
	testPhotoID = "22222222-2222-2222-2222-222222222222"
	testUserID  = "22222222-2222-2222-2222-222222222222"
)

// ---- 公开接口 ----

func TestListPublishedAlbumsReturnsPageEnvelope(t *testing.T) {
	publishedAt := pgtype.Timestamptz{Time: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Valid: true}
	store := &albumStoreStub{
		publishedTotal: 57,
		publishedRows: []dbgen.ListPublishedPhotoAlbumsRow{
			{
				AlbumID:     uuidForTest(testAlbumID),
				Slug:        "city-nights",
				Title:       "城市夜色",
				TitleEn:     pgtype.Text{String: "City Nights", Valid: true},
				Description: "一组夜景。",
				Tags:        []string{"photography", "city"},
				PublishedAt: publishedAt,
				PhotoCount:  87,
				CoverUrl:    pgtype.Text{String: "https://files.example.com/photos/cover.jpg", Valid: true},
				CoverWidth:  2400,
				CoverHeight: 1600,
			},
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/photos?page=1&limit=12", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := jsonDecode(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["page"].(float64) != 1 || body["limit"].(float64) != 12 || body["total"].(float64) != 57 || body["totalPages"].(float64) != 5 {
		t.Fatalf("pagination metadata = %#v", body)
	}
	data := body["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("data length = %d, want 1", len(data))
	}
	album := data[0].(map[string]interface{})
	for _, key := range []string{"slug", "title", "titleEn", "description", "descriptionEn", "coverUrl", "coverWidth", "coverHeight", "publishedAt", "tags", "photoCount"} {
		if _, exists := album[key]; !exists {
			t.Fatalf("album item missing contract field %q: %#v", key, album)
		}
	}
	for _, forbidden := range []string{"albumId", "status", "createdBy", "createdAt", "updatedAt", "coverPhotoId"} {
		if _, exists := album[forbidden]; exists {
			t.Fatalf("public album item must not expose %q", forbidden)
		}
	}
	if album["slug"] != "city-nights" || album["photoCount"].(float64) != 87 {
		t.Fatalf("unexpected album item: %#v", album)
	}
	if rec.Header().Get("ETag") == "" || rec.Header().Get("Cache-Control") == "" {
		t.Fatalf("public responses must carry ETag and Cache-Control")
	}
}

func TestListPublishedAlbumsEmptyPageOutOfRange(t *testing.T) {
	store := &albumStoreStub{publishedTotal: 5}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/photos?page=9&limit=12", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with empty data", rec.Code)
	}
	var body map[string]interface{}
	if err := jsonDecode(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body["data"].([]interface{})) != 0 {
		t.Fatalf("out-of-range page must return empty data: %#v", body["data"])
	}
	if body["total"].(float64) != 5 || body["totalPages"].(float64) != 1 {
		t.Fatalf("metadata = %#v", body)
	}
}

func TestListPublishedAlbumsRejectsInvalidPagination(t *testing.T) {
	handler := NewHandler(&albumStoreStub{})
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	for _, query := range []string{"page=0", "page=-1", "page=abc", "limit=0", "limit=-5", "limit=abc"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/photos?"+query, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d, want 400", query, rec.Code)
		}
	}
}

func TestListPublishedAlbumsClampsLimitToMax(t *testing.T) {
	store := &albumStoreStub{publishedTotal: 1}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/photos?limit=500", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if store.lastListLimit != 100 {
		t.Fatalf("store limit = %d, want clamped 100", store.lastListLimit)
	}
	var body map[string]interface{}
	if err := jsonDecode(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["limit"].(float64) != 100 {
		t.Fatalf("response limit = %#v, want 100", body["limit"])
	}
}

func TestPublicAlbumEndpointsReturnNotFound(t *testing.T) {
	store := &albumStoreStub{
		publishedAlbumErr: pgx.ErrNoRows,
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	detail := httptest.NewRecorder()
	router.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/v1/photos/missing-album", nil))
	if detail.Code != http.StatusNotFound {
		t.Fatalf("detail status = %d, want 404", detail.Code)
	}
	if detail.Header().Get("Cache-Control") != "public, max-age=15" {
		t.Fatalf("404 should carry short cache header, got %q", detail.Header().Get("Cache-Control"))
	}

	photos := httptest.NewRecorder()
	router.ServeHTTP(photos, httptest.NewRequest(http.MethodGet, "/api/v1/photos/missing-album/photos", nil))
	if photos.Code != http.StatusNotFound {
		t.Fatalf("photos status = %d, want 404", photos.Code)
	}

	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/photos/Invalid_Slug", nil))
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("invalid slug status = %d, want 404", invalid.Code)
	}
}

func TestGetPublishedAlbumSupportsETagRevalidation(t *testing.T) {
	store := &albumStoreStub{
		publishedAlbum: dbgen.GetPublishedPhotoAlbumBySlugRow{
			AlbumID:     uuidForTest(testAlbumID),
			Slug:        "city-nights",
			Title:       "城市夜色",
			Description: "一组夜景。",
			Tags:        []string{},
			PublishedAt: pgtype.Timestamptz{Time: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Valid: true},
			PhotoCount:  3,
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/photos/city-nights", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d", first.Code)
	}
	etag := first.Header().Get("ETag")

	revalidated := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/photos/city-nights", nil)
	request.Header.Set("If-None-Match", etag)
	router.ServeHTTP(revalidated, request)
	if revalidated.Code != http.StatusNotModified {
		t.Fatalf("revalidation status = %d, want 304", revalidated.Code)
	}
}

func TestListPublishedAlbumPhotosReturnsContractFields(t *testing.T) {
	store := &albumStoreStub{
		publishedAlbum: dbgen.GetPublishedPhotoAlbumBySlugRow{
			AlbumID:     uuidForTest(testAlbumID),
			Slug:        "city-nights",
			Title:       "城市夜色",
			Description: "一组夜景。",
			PhotoCount:  87,
		},
		photoTotal: 87,
		photoRows: []dbgen.ListPublishedAlbumPhotosBySlugRow{
			{
				PhotoID: uuidForTest(testPhotoID),
				Alt:     "夜色中的天桥",
				Width:   2400,
				Height:  1600,
				Url:     pgtype.Text{String: "https://files.example.com/photos/01.jpg", Valid: true},
				TakenAt: pgtype.Timestamptz{Time: time.Date(2026, 10, 1, 18, 42, 0, 0, time.UTC), Valid: true},
			},
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicReadRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/photos/city-nights/photos?page=1&limit=24", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := jsonDecode(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["total"].(float64) != 87 || body["totalPages"].(float64) != 4 {
		t.Fatalf("metadata = %#v", body)
	}
	photo := body["data"].([]interface{})[0].(map[string]interface{})
	for _, key := range []string{"url", "alt", "caption", "width", "height", "takenAt"} {
		if _, exists := photo[key]; !exists {
			t.Fatalf("photo item missing contract field %q", key)
		}
	}
	for _, forbidden := range []string{"photoId", "position", "filename", "contentType"} {
		if _, exists := photo[forbidden]; exists {
			t.Fatalf("public photo item must not expose %q", forbidden)
		}
	}
}

// ---- 后台接口 ----

func TestAdminAlbumRoutesRequireAuth(t *testing.T) {
	handler := NewHandler(&albumStoreStub{})
	router := chi.NewRouter()
	router.Use(httpapi.AdminAuthMiddleware("test-secret"))
	handler.RegisterAdminRoutes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/albums", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("list status = %d, want 401", rec.Code)
	}
}

func TestCreateAlbumValidatesPayload(t *testing.T) {
	router := authenticatedAlbumRouter(t, NewHandler(&albumStoreStub{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authRequest(t, http.MethodPost, "/albums", map[string]interface{}{
		"slug":        "City Nights",
		"title":       "城市夜色",
		"description": "一组夜景。",
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid slug status = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authRequest(t, http.MethodPost, "/albums", map[string]interface{}{
		"slug":        "city-nights",
		"title":       "",
		"description": "一组夜景。",
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing title status = %d, want 400", rec.Code)
	}
}

func TestPublishAlbumRequiresAtLeastOnePhoto(t *testing.T) {
	store := &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{
			AlbumID: uuidForTest(testAlbumID),
			Slug:    "city-nights",
			Status:  dbgen.PhotoAlbumStatusDraft,
		},
		photoCountByID: 0,
	}
	router := authenticatedAlbumRouter(t, NewHandler(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authRequest(t, http.MethodPost, "/albums/"+testAlbumID+"/publish", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("publish empty album status = %d, want 409, body: %s", rec.Code, rec.Body.String())
	}
}

func TestPublishedAlbumSlugImmutable(t *testing.T) {
	store := &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{
			AlbumID: uuidForTest(testAlbumID),
			Slug:    "city-nights",
			Status:  dbgen.PhotoAlbumStatusPublished,
		},
	}
	router := authenticatedAlbumRouter(t, NewHandler(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authRequest(t, http.MethodPatch, "/albums/"+testAlbumID, map[string]interface{}{
		"slug":        "city-nights-renamed",
		"title":       "城市夜色",
		"description": "一组夜景。",
	}))
	if rec.Code != http.StatusConflict {
		t.Fatalf("rename published album status = %d, want 409", rec.Code)
	}
}

func TestUploadAlbumPhotoRejectsNonImage(t *testing.T) {
	router := authenticatedAlbumRouter(t, newUploadCapableHandler(t, &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{
			AlbumID: uuidForTest(testAlbumID),
			Slug:    "city-nights",
			Status:  dbgen.PhotoAlbumStatusDraft,
		},
	}))

	req := multipartPhotoRequest(t, "/albums/"+testAlbumID+"/photos", map[string]string{"alt": "测试"}, "note.txt", []byte("plain text"), "text/plain")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-image upload status = %d, want 400, body: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadAlbumPhotoRequiresAlt(t *testing.T) {
	router := authenticatedAlbumRouter(t, newUploadCapableHandler(t, &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{
			AlbumID: uuidForTest(testAlbumID),
			Slug:    "city-nights",
			Status:  dbgen.PhotoAlbumStatusDraft,
		},
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, multipartPhotoRequest(t, "/albums/"+testAlbumID+"/photos", nil, "shot.png", encodeTestPNG(t, 4, 3), "image/png"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing alt status = %d, want 400, body: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadAlbumPhotoStoresProcessedImage(t *testing.T) {
	store := &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{
			AlbumID: uuidForTest(testAlbumID),
			Slug:    "city-nights",
			Status:  dbgen.PhotoAlbumStatusDraft,
		},
	}
	handler, objects := newUploadHandler(t, store)
	router := authenticatedAlbumRouter(t, handler)

	pngBytes := encodeTestPNG(t, 8, 6)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, multipartPhotoRequest(t, "/albums/"+testAlbumID+"/photos", map[string]string{"alt": "夜色中的天桥"}, "shot.png", pngBytes, "image/png"))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if len(objects.putCalls) != 1 {
		t.Fatalf("put calls = %d, want 1", len(objects.putCalls))
	}
	call := objects.putCalls[0]
	if call.contentType != "image/png" {
		t.Fatalf("content type = %q", call.contentType)
	}
	if call.cacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("cache control = %q", call.cacheControl)
	}
	if len(call.key) <= len("photos/") || call.key[:7] != "photos/" {
		t.Fatalf("storage key = %q, want photos/ prefix", call.key)
	}
	if !bytes.Equal(call.body, pngBytes) {
		t.Fatal("stored body should match cleaned png bytes")
	}
	if store.lastAssetCreate.Width.Int32 != 8 || store.lastAssetCreate.Height.Int32 != 6 {
		t.Fatalf("asset dims = %dx%d, want 8x6", store.lastAssetCreate.Width.Int32, store.lastAssetCreate.Height.Int32)
	}
	if store.lastPhotoCreate.Alt != "夜色中的天桥" {
		t.Fatalf("photo alt = %q", store.lastPhotoCreate.Alt)
	}

	var body map[string]interface{}
	if err := jsonDecode(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["width"].(float64) != 8 || body["height"].(float64) != 6 {
		t.Fatalf("response dims = %#v", body)
	}
	if body["url"] != "https://files.example.com/"+call.key {
		t.Fatalf("response url = %#v, want key-based public url", body["url"])
	}
}

func TestUploadAlbumPhotoRequiresPublicBaseURL(t *testing.T) {
	store := &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{
			AlbumID: uuidForTest(testAlbumID),
			Slug:    "city-nights",
		},
	}
	handler := NewHandlerWithOptions(store, &fakeObjectStore{}, HandlerOptions{PhotoPrefix: "photos"})
	router := authenticatedAlbumRouter(t, handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, multipartPhotoRequest(t, "/albums/"+testAlbumID+"/photos", map[string]string{"alt": "x"}, "shot.png", encodeTestPNG(t, 2, 2), "image/png"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 without public base url", rec.Code)
	}
}

func TestReorderAlbumPhotosRequiresFullSet(t *testing.T) {
	store := &albumStoreStub{
		albumByID: dbgen.GetPhotoAlbumByIDRow{AlbumID: uuidForTest(testAlbumID), Status: dbgen.PhotoAlbumStatusDraft},
		albumPhotos: []dbgen.ListAlbumPhotosRow{
			{PhotoID: uuidForTest(testPhotoID)},
			{PhotoID: uuidForTest("33333333-3333-3333-3333-333333333333")},
		},
	}
	router := authenticatedAlbumRouter(t, NewHandler(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authRequest(t, http.MethodPut, "/albums/"+testAlbumID+"/photos/order", map[string]interface{}{
		"photoIds": []string{testPhotoID},
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("partial reorder status = %d, want 400", rec.Code)
	}
}

func TestSetAlbumCoverValidatesPhotoInAlbum(t *testing.T) {
	store := &albumStoreStub{
		albumByID:    dbgen.GetPhotoAlbumByIDRow{AlbumID: uuidForTest(testAlbumID), Status: dbgen.PhotoAlbumStatusDraft},
		photoByIDErr: pgx.ErrNoRows,
	}
	router := authenticatedAlbumRouter(t, NewHandler(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authRequest(t, http.MethodPost, "/albums/"+testAlbumID+"/cover", map[string]interface{}{
		"photoId": testPhotoID,
	}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign cover photo status = %d, want 404", rec.Code)
	}
}

// ---- 测试助手 ----

func authenticatedAlbumRouter(t *testing.T, handler Handler) chi.Router {
	t.Helper()
	router := chi.NewRouter()
	router.Use(httpapi.AdminAuthMiddleware("test-secret"))
	handler.RegisterAdminRoutes(router)
	return router
}

func adminToken(t *testing.T) string {
	t.Helper()
	token, err := auth.IssueToken("test-secret", testUserID, "admin")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func newUploadHandler(t *testing.T, store *albumStoreStub) (Handler, *fakeObjectStore) {
	t.Helper()
	objects := &fakeObjectStore{}
	handler := NewHandlerWithOptions(store, objects, HandlerOptions{
		PublicBaseURL: "https://files.example.com",
		PhotoPrefix:   "photos",
	})
	return handler, objects
}

func newUploadCapableHandler(t *testing.T, store *albumStoreStub) Handler {
	handler, _ := newUploadHandler(t, store)
	return handler
}

func encodeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 0x22, G: 0x44, B: 0x66, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func multipartPhotoRequest(t *testing.T, target string, fields map[string]string, filename string, content []byte, contentType string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	_ = contentType
	return req
}

func jsonDecode(data []byte, target *map[string]interface{}) error {
	return jsonUnmarshal(data, target)
}

type fakeObjectStore struct {
	putCalls    []fakePutCall
	deleteCalls []string
}

type fakePutCall struct {
	key          string
	body         []byte
	contentType  string
	cacheControl string
}

func (s *fakeObjectStore) PutObject(ctx context.Context, key string, body []byte, contentType string, cacheControl string) error {
	s.putCalls = append(s.putCalls, fakePutCall{key: key, body: append([]byte{}, body...), contentType: contentType, cacheControl: cacheControl})
	return nil
}

func (s *fakeObjectStore) DeleteObject(ctx context.Context, key string) error {
	s.deleteCalls = append(s.deleteCalls, key)
	return nil
}
