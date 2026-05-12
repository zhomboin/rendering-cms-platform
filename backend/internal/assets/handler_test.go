package assets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	updateStatusArg dbgen.UpdateAssetStatusParams
}

func (s *assetStoreStub) ListAssets(ctx context.Context) ([]dbgen.Asset, error) {
	return nil, nil
}

func (s *assetStoreStub) CreateAsset(ctx context.Context, arg dbgen.CreateAssetParams) (dbgen.Asset, error) {
	return dbgen.Asset{}, nil
}

func (s *assetStoreStub) GetAssetByID(ctx context.Context, assetID pgtype.UUID) (dbgen.Asset, error) {
	return dbgen.Asset{}, nil
}

func (s *assetStoreStub) CreateDownloadEvent(ctx context.Context, arg dbgen.CreateDownloadEventParams) (dbgen.DownloadEvent, error) {
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
