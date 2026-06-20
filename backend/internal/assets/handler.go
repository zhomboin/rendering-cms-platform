package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

const presignExpiry = 15 * time.Minute

const (
	UploadUsageAssetFile = "asset-file"
	UploadUsageBlogImage = "blog-image"
)

type URLSigner interface {
	PresignUploadURL(ctx context.Context, key string, contentType string, byteSize int64, expires time.Duration) (string, error)
	PresignDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

type Handler struct {
	queries assetStore
	signer  URLSigner
	options HandlerOptions
}

type HandlerOptions struct {
	PublicBaseURL   string
	BlogImagePrefix string
	AssetFilePrefix string
	Now             func() time.Time
}

type UploadURLPayload struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	ByteSize    int    `json:"byteSize"`
	Usage       string `json:"usage"`
}

type UpdateAssetStatusPayload struct {
	Status string `json:"status"`
}

type assetStore interface {
	ListAssets(ctx context.Context) ([]dbgen.Asset, error)
	CreateAsset(ctx context.Context, arg dbgen.CreateAssetParams) (dbgen.Asset, error)
	GetAssetByID(ctx context.Context, assetID pgtype.UUID) (dbgen.Asset, error)
	CreateDownloadEvent(ctx context.Context, arg dbgen.CreateDownloadEventParams) (dbgen.DownloadEvent, error)
	UpdateAssetStatus(ctx context.Context, arg dbgen.UpdateAssetStatusParams) (dbgen.Asset, error)
}

func NewHandler(queries assetStore, signer URLSigner) Handler {
	return NewHandlerWithOptions(queries, signer, HandlerOptions{})
}

func NewHandlerWithOptions(queries assetStore, signer URLSigner, options HandlerOptions) Handler {
	if options.Now == nil {
		options.Now = time.Now
	}
	options.PublicBaseURL = strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/")
	options.BlogImagePrefix = cleanStoragePrefix(options.BlogImagePrefix, "blog")
	options.AssetFilePrefix = cleanStoragePrefix(options.AssetFilePrefix, "assets")
	return Handler{queries: queries, signer: signer, options: options}
}

func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/assets", h.listAssets)
	router.Post("/assets/upload-url", h.createUploadURL)
	router.Get("/assets/{id}/download-url", h.createDownloadURL)
	router.Patch("/assets/{id}", h.updateAssetStatus)
}

func (h Handler) listAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := h.queries.ListAssets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "资源列表读取失败")
		return
	}
	response := make([]map[string]interface{}, 0, len(assets))
	for _, asset := range assets {
		response = append(response, mapAsset(asset))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) createUploadURL(w http.ResponseWriter, r *http.Request) {
	if h.signer == nil {
		writeError(w, http.StatusServiceUnavailable, "对象存储未配置")
		return
	}
	// 后台文件上传必须绑定当前登录用户，避免未认证用户创建资源记录或申请 R2 上传 URL。
	user, ok := httpapi.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return
	}
	var payload UploadURLPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	payload.Filename = strings.TrimSpace(payload.Filename)
	payload.ContentType = strings.TrimSpace(payload.ContentType)
	payload.Usage = normalizeUploadUsage(payload.Usage)
	// 在生成 R2 预签名 URL 前先校验文件名、类型和大小，确保对象存储侧只接收允许的文件。
	if err := ValidateUpload(payload.Filename, payload.ContentType, payload.ByteSize); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Usage == UploadUsageBlogImage && !strings.HasPrefix(payload.ContentType, "image/") {
		writeError(w, http.StatusBadRequest, "文章图片只能上传图片类型")
		return
	}
	userID, err := uuidFromString(user.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "用户 ID 无效")
		return
	}

	storageKey := h.storageKeyFor(payload.Filename, payload.Usage)
	publicURL := h.publicURLFor(storageKey, payload.Usage)
	// 先写入资源元数据和 storage_key；文件本体随后由前端通过预签名 URL 直传到 R2。
	asset, err := h.queries.CreateAsset(r.Context(), dbgen.CreateAssetParams{
		Filename:    payload.Filename,
		ContentType: payload.ContentType,
		ByteSize:    int32(payload.ByteSize),
		StorageKey:  storageKey,
		PublicUrl:   nullableText(publicURL),
		CreatedBy:   userID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "资源记录创建失败")
		return
	}
	// 返回给浏览器的是短期 PUT URL，不包含 R2 密钥；实际上传由前端直接 PUT 到该 URL。
	uploadURL, err := h.signer.PresignUploadURL(r.Context(), storageKey, payload.ContentType, int64(payload.ByteSize), presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "上传 URL 生成失败")
		return
	}

	// headers 必须和签名时的输入一致，否则 R2/S3 会拒绝该预签名请求。
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"asset":     mapAsset(asset),
		"uploadUrl": uploadURL,
		"method":    http.MethodPut,
		"headers": map[string]string{
			"Content-Type": payload.ContentType,
		},
		"expiresInSeconds": int(presignExpiry.Seconds()),
	})
}

func (h Handler) createDownloadURL(w http.ResponseWriter, r *http.Request) {
	if h.signer == nil {
		writeError(w, http.StatusServiceUnavailable, "对象存储未配置")
		return
	}
	assetID, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "资源 ID 无效")
		return
	}
	asset, err := h.queries.GetAssetByID(r.Context(), assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "资源不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "资源读取失败")
		return
	}
	downloadURL, err := h.signer.PresignDownloadURL(r.Context(), asset.StorageKey, presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "下载 URL 生成失败")
		return
	}
	if _, err := h.queries.CreateDownloadEvent(r.Context(), dbgen.CreateDownloadEventParams{
		AssetID:   asset.AssetID,
		IpHash:    ipHashFromRequest(r),
		UserAgent: nullableText(r.UserAgent()),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "下载审计写入失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"asset":            mapAsset(asset),
		"downloadUrl":      downloadURL,
		"expiresInSeconds": int(presignExpiry.Seconds()),
	})
}

func (h Handler) updateAssetStatus(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "资源 ID 无效")
		return
	}
	var payload UpdateAssetStatusPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	status := strings.TrimSpace(payload.Status)
	if !ValidAssetStatus(status) {
		writeError(w, http.StatusBadRequest, "资源状态只能为 active、archived 或 deleted")
		return
	}
	deletedAt := pgtype.Timestamptz{}
	if status == StatusDeleted {
		deletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	asset, err := h.queries.UpdateAssetStatus(r.Context(), dbgen.UpdateAssetStatusParams{
		AssetID:   assetID,
		Status:    dbgen.AssetStatus(status),
		DeletedAt: deletedAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "资源不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "资源状态更新失败")
		return
	}
	writeJSON(w, http.StatusOK, mapAsset(asset))
}

func (h Handler) storageKeyFor(filename string, usage string) string {
	now := h.options.Now().UTC()
	prefix := h.options.AssetFilePrefix
	if usage == UploadUsageBlogImage {
		prefix = h.options.BlogImagePrefix
	}
	extension := strings.ToLower(path.Ext(path.Base(strings.ReplaceAll(filename, "\\", "/"))))
	return path.Join(prefix, fmt.Sprintf("%04d", now.Year()), fmt.Sprintf("%02d", int(now.Month())), uuid.NewString()+extension)
}

func (h Handler) publicURLFor(storageKey string, usage string) string {
	if usage != UploadUsageBlogImage || h.options.PublicBaseURL == "" {
		return ""
	}
	return h.options.PublicBaseURL + "/" + strings.TrimLeft(storageKey, "/")
}

func cleanStoragePrefix(value string, fallback string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" || value == "." {
		return fallback
	}
	return value
}

func normalizeUploadUsage(value string) string {
	value = strings.TrimSpace(value)
	if value == UploadUsageBlogImage {
		return UploadUsageBlogImage
	}
	return UploadUsageAssetFile
}

func uuidFromString(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return uuid, nil
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func ipHashFromRequest(r *http.Request) string {
	return httpapi.ClientIPHash(r)
}

func mapAsset(asset dbgen.Asset) map[string]interface{} {
	return map[string]interface{}{
		"assetId":     asset.AssetID.String(),
		"filename":    asset.Filename,
		"contentType": asset.ContentType,
		"byteSize":    asset.ByteSize,
		"publicUrl":   textValue(asset.PublicUrl),
		"createdBy":   asset.CreatedBy.String(),
		"createdAt":   timestamptzValue(asset.CreatedAt),
		"status":      string(asset.Status),
		"deletedAt":   timestamptzValue(asset.DeletedAt),
	}
}

func textValue(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
