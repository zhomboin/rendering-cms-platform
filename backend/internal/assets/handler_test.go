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

	"rendering-cms-platform/backend/internal/database/dbgen"
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
	downloadEventIPHash string
}

func (s *assetStoreStub) ListAssets(ctx context.Context) ([]dbgen.Asset, error) {
	return nil, nil
}

func (s *assetStoreStub) CreateAsset(ctx context.Context, arg dbgen.CreateAssetParams) (dbgen.Asset, error) {
	return dbgen.Asset{}, nil
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

type urlSignerStub struct{}

func (s urlSignerStub) PresignUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error) {
	return "https://example.com/upload", nil
}

func (s urlSignerStub) PresignDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return "https://example.com/download", nil
}

func TestCreateDownloadURLUsesForwardedClientIPHash(t *testing.T) {
	store := &assetStoreStub{}
	handler := NewHandler(store, urlSignerStub{})
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
