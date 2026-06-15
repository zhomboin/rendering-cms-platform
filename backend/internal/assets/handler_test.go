package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

func TestUpdateAssetStatusSoftDeletesAsset(t *testing.T) {
	store := &assetStoreStub{}
	handler := NewHandler(store, nil)
	router := chi.NewRouter()
	handler.RegisterAdminRoutes(router)
	req := httptest.NewRequest(http.MethodPatch, "/assets/11111111-1111-1111-1111-111111111111", strings.NewReader(`{
		"status": "deleted"
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.updateStatusArg.Status != dbgen.AssetStatusDeleted {
		t.Fatalf("status = %q, want deleted", store.updateStatusArg.Status)
	}
	if !store.updateStatusArg.DeletedAt.Valid {
		t.Fatal("deleted_at should be set for deleted status")
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "deleted" {
		t.Fatalf("response status = %#v, want deleted", body["status"])
	}
}

type assetStoreStub struct {
	updateStatusArg     dbgen.UpdateAssetStatusParams
	createAssetArg      dbgen.CreateAssetParams
	downloadEventIPHash string
}

func (s *assetStoreStub) ListAssets(ctx context.Context) ([]dbgen.Asset, error) {
	return nil, nil
}

func (s *assetStoreStub) CreateAsset(ctx context.Context, arg dbgen.CreateAssetParams) (dbgen.Asset, error) {
	s.createAssetArg = arg
	return dbgen.Asset{
		Filename:    arg.Filename,
		ContentType: arg.ContentType,
		ByteSize:    arg.ByteSize,
		StorageKey:  arg.StorageKey,
		PublicUrl:   arg.PublicUrl,
		CreatedBy:   arg.CreatedBy,
		Status:      dbgen.AssetStatusActive,
	}, nil
}

func (s *assetStoreStub) GetAssetByID(ctx context.Context, assetID pgtype.UUID) (dbgen.Asset, error) {
	return dbgen.Asset{
		AssetID:    assetID,
		Filename:   "diagram.webp",
		StorageKey: "assets/example/diagram.webp",
		Status:     dbgen.AssetStatusActive,
	}, nil
}

func (s *assetStoreStub) CreateDownloadEvent(ctx context.Context, arg dbgen.CreateDownloadEventParams) (dbgen.DownloadEvent, error) {
	s.downloadEventIPHash = arg.IpHash
	return dbgen.DownloadEvent{}, nil
}

func (s *assetStoreStub) UpdateAssetStatus(ctx context.Context, arg dbgen.UpdateAssetStatusParams) (dbgen.Asset, error) {
	s.updateStatusArg = arg
	return dbgen.Asset{
		AssetID:     arg.AssetID,
		Filename:    "diagram.webp",
		ContentType: "image/webp",
		ByteSize:    1024,
		StorageKey:  "assets/example/diagram.webp",
		Status:      arg.Status,
		DeletedAt:   arg.DeletedAt,
	}, nil
}

type urlSignerStub struct {
	uploadKey         string
	uploadContentType string
}

func (s *urlSignerStub) PresignUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error) {
	s.uploadKey = key
	s.uploadContentType = contentType
	return "https://example.com/upload", nil
}

func (s *urlSignerStub) PresignDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return "https://example.com/download", nil
}

func TestCreateUploadURLUsesBlogImagePublicURLAndDatedStorageKey(t *testing.T) {
	store := &assetStoreStub{}
	signer := &urlSignerStub{}
	handler := NewHandlerWithOptions(store, signer, HandlerOptions{
		PublicBaseURL:   "https://assets.rendering.me/media/",
		BlogImagePrefix: "blog",
		AssetFilePrefix: "files",
		Now: func() time.Time {
			return time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
		},
	})
	router := chi.NewRouter()
	router.With(httpapi.AdminAuthMiddleware("secret-32-characters-minimum-value")).Post("/assets/upload-url", handler.createUploadURL)
	req := httptest.NewRequest(http.MethodPost, "/assets/upload-url", strings.NewReader(`{
		"filename": "Demo Image.webp",
		"contentType": "image/webp",
		"byteSize": 2048,
		"usage": "blog-image"
	}`))
	token, err := auth.IssueToken("secret-32-characters-minimum-value", "11111111-1111-1111-1111-111111111111", "admin")
	if err != nil {
		t.Fatalf("IssueToken() returned error: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.HasPrefix(store.createAssetArg.StorageKey, "blog/2026/06/") {
		t.Fatalf("storage key = %q, want blog year/month prefix", store.createAssetArg.StorageKey)
	}
	if !strings.HasSuffix(store.createAssetArg.StorageKey, ".webp") {
		t.Fatalf("storage key = %q, want original extension", store.createAssetArg.StorageKey)
	}
	if strings.Contains(store.createAssetArg.StorageKey, "Demo") {
		t.Fatalf("storage key = %q, should not include original filename", store.createAssetArg.StorageKey)
	}
	wantPublicURL := "https://assets.rendering.me/media/" + store.createAssetArg.StorageKey
	if !store.createAssetArg.PublicUrl.Valid || store.createAssetArg.PublicUrl.String != wantPublicURL {
		t.Fatalf("public url = %#v, want %q", store.createAssetArg.PublicUrl, wantPublicURL)
	}
	if signer.uploadKey != store.createAssetArg.StorageKey {
		t.Fatalf("signed key = %q, want storage key %q", signer.uploadKey, store.createAssetArg.StorageKey)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	asset := body["asset"].(map[string]interface{})
	if asset["publicUrl"] != wantPublicURL {
		t.Fatalf("response publicUrl = %#v, want %q", asset["publicUrl"], wantPublicURL)
	}
}

func TestCreateUploadURLUsesAssetPrefixAndNoPublicURLByDefault(t *testing.T) {
	store := &assetStoreStub{}
	signer := &urlSignerStub{}
	handler := NewHandlerWithOptions(store, signer, HandlerOptions{
		PublicBaseURL:   "https://assets.rendering.me",
		BlogImagePrefix: "blog",
		AssetFilePrefix: "files",
		Now: func() time.Time {
			return time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
		},
	})
	router := chi.NewRouter()
	router.With(httpapi.AdminAuthMiddleware("secret-32-characters-minimum-value")).Post("/assets/upload-url", handler.createUploadURL)
	req := httptest.NewRequest(http.MethodPost, "/assets/upload-url", strings.NewReader(`{
		"filename": "report.pdf",
		"contentType": "application/pdf",
		"byteSize": 4096
	}`))
	token, err := auth.IssueToken("secret-32-characters-minimum-value", "11111111-1111-1111-1111-111111111111", "admin")
	if err != nil {
		t.Fatalf("IssueToken() returned error: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.HasPrefix(store.createAssetArg.StorageKey, "files/2026/06/") {
		t.Fatalf("storage key = %q, want files year/month prefix", store.createAssetArg.StorageKey)
	}
	if !strings.HasSuffix(store.createAssetArg.StorageKey, ".pdf") {
		t.Fatalf("storage key = %q, want original extension", store.createAssetArg.StorageKey)
	}
	if store.createAssetArg.PublicUrl.Valid {
		t.Fatalf("public url = %#v, want null for default asset upload", store.createAssetArg.PublicUrl)
	}
}

func TestCreateDownloadURLUsesForwardedClientIPHash(t *testing.T) {
	store := &assetStoreStub{}
	handler := NewHandler(store, &urlSignerStub{})
	router := chi.NewRouter()
	handler.RegisterAdminRoutes(router)
	req := httptest.NewRequest(http.MethodGet, "/assets/11111111-1111-1111-1111-111111111111/download-url", nil)
	req.RemoteAddr = "10.0.0.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.10")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	want := sha256.Sum256([]byte("203.0.113.10"))
	if store.downloadEventIPHash != hex.EncodeToString(want[:]) {
		t.Fatalf("ip hash = %q, want forwarded client hash", store.downloadEventIPHash)
	}
}
