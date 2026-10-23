package albums

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

// 公开相册接口的分页默认值与上限；limit 超过上限时按上限处理。
const (
	defaultAlbumPageLimit = 12
	defaultPhotoPageLimit = 24
	maxPageLimit          = 100
)

// photoObjectCacheControl 写入对象存储元数据，公开访问照片时原样返回长缓存头。
const photoObjectCacheControl = "public, max-age=31536000, immutable"

var errAlbumNotFound = errors.New("album not found")

// ObjectStore 抽象相册照片需要的对象存储写入与删除能力。
type ObjectStore interface {
	PutObject(ctx context.Context, key string, body []byte, contentType string, cacheControl string) error
	DeleteObject(ctx context.Context, key string) error
}

type Handler struct {
	queries albumStore
	store   ObjectStore
	options HandlerOptions
}

type HandlerOptions struct {
	PublicBaseURL string
	PhotoPrefix   string
	Now           func() time.Time
}

type albumStore interface {
	CountPublishedPhotoAlbums(ctx context.Context) (int32, error)
	ListPublishedPhotoAlbums(ctx context.Context, arg dbgen.ListPublishedPhotoAlbumsParams) ([]dbgen.ListPublishedPhotoAlbumsRow, error)
	GetPublishedPhotoAlbumBySlug(ctx context.Context, slug string) (dbgen.GetPublishedPhotoAlbumBySlugRow, error)
	CountPublishedAlbumPhotosBySlug(ctx context.Context, slug string) (int32, error)
	ListPublishedAlbumPhotosBySlug(ctx context.Context, arg dbgen.ListPublishedAlbumPhotosBySlugParams) ([]dbgen.ListPublishedAlbumPhotosBySlugRow, error)

	ListAdminPhotoAlbums(ctx context.Context) ([]dbgen.ListAdminPhotoAlbumsRow, error)
	GetPhotoAlbumByID(ctx context.Context, albumID pgtype.UUID) (dbgen.GetPhotoAlbumByIDRow, error)
	CreatePhotoAlbum(ctx context.Context, arg dbgen.CreatePhotoAlbumParams) (dbgen.PhotoAlbum, error)
	UpdatePhotoAlbum(ctx context.Context, arg dbgen.UpdatePhotoAlbumParams) (dbgen.PhotoAlbum, error)
	PublishPhotoAlbum(ctx context.Context, albumID pgtype.UUID) (dbgen.PhotoAlbum, error)
	UnpublishPhotoAlbum(ctx context.Context, albumID pgtype.UUID) (dbgen.PhotoAlbum, error)
	SetPhotoAlbumCover(ctx context.Context, arg dbgen.SetPhotoAlbumCoverParams) (dbgen.PhotoAlbum, error)
	DeletePhotoAlbum(ctx context.Context, albumID pgtype.UUID) error
	CountAlbumPhotosByID(ctx context.Context, albumID pgtype.UUID) (int32, error)

	ListAlbumPhotos(ctx context.Context, albumID pgtype.UUID) ([]dbgen.ListAlbumPhotosRow, error)
	GetAlbumPhotoByID(ctx context.Context, arg dbgen.GetAlbumPhotoByIDParams) (dbgen.GetAlbumPhotoByIDRow, error)
	CreateAlbumPhotoAsset(ctx context.Context, arg dbgen.CreateAlbumPhotoAssetParams) (dbgen.Asset, error)
	CreateAlbumPhoto(ctx context.Context, arg dbgen.CreateAlbumPhotoParams) (dbgen.AlbumPhoto, error)
	UpdateAlbumPhoto(ctx context.Context, arg dbgen.UpdateAlbumPhotoParams) (dbgen.AlbumPhoto, error)
	DeleteAlbumPhoto(ctx context.Context, arg dbgen.DeleteAlbumPhotoParams) (int64, error)
	ClearPhotoAlbumCoverIfRemoved(ctx context.Context, arg dbgen.ClearPhotoAlbumCoverIfRemovedParams) error
	ReorderAlbumPhotos(ctx context.Context, arg dbgen.ReorderAlbumPhotosParams) error
	ListAlbumAssetKeys(ctx context.Context, albumID pgtype.UUID) ([]dbgen.ListAlbumAssetKeysRow, error)
	SoftDeleteAlbumAssets(ctx context.Context, albumID pgtype.UUID) (int64, error)
	UpdateAssetStatus(ctx context.Context, arg dbgen.UpdateAssetStatusParams) (dbgen.Asset, error)
}

func NewHandler(queries albumStore) Handler {
	return NewHandlerWithOptions(queries, nil, HandlerOptions{})
}

func NewHandlerWithOptions(queries albumStore, store ObjectStore, options HandlerOptions) Handler {
	if options.Now == nil {
		options.Now = time.Now
	}
	options.PublicBaseURL = strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/")
	options.PhotoPrefix = cleanStoragePrefix(options.PhotoPrefix, "photos")
	return Handler{queries: queries, store: store, options: options}
}

func (h Handler) RegisterPublicReadRoutes(router chi.Router) {
	router.Get("/api/v1/photos", h.listPublishedAlbums)
	router.Get("/api/v1/photos/{slug}", h.getPublishedAlbum)
	router.Get("/api/v1/photos/{slug}/photos", h.listPublishedAlbumPhotos)
}

func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/albums", h.listAdminAlbums)
	router.Post("/albums", h.createAlbum)
	router.Get("/albums/{id}", h.getAdminAlbum)
	router.Patch("/albums/{id}", h.updateAlbum)
	router.Post("/albums/{id}/publish", h.publishAlbum)
	router.Post("/albums/{id}/unpublish", h.unpublishAlbum)
	router.Delete("/albums/{id}", h.deleteAlbum)
	router.Get("/albums/{id}/photos", h.listAdminAlbumPhotos)
	router.Post("/albums/{id}/photos", h.uploadAlbumPhoto)
	router.Patch("/albums/{id}/photos/{photoId}", h.updateAlbumPhoto)
	router.Delete("/albums/{id}/photos/{photoId}", h.deleteAlbumPhoto)
	router.Put("/albums/{id}/photos/order", h.reorderAlbumPhotos)
	router.Post("/albums/{id}/cover", h.setAlbumCover)
}

// ---- 公开接口 ----

func (h Handler) listPublishedAlbums(w http.ResponseWriter, r *http.Request) {
	pageParams, ok := parsePagination(r, defaultAlbumPageLimit)
	if !ok {
		writePublicPhotoError(w, http.StatusBadRequest, "分页参数不正确")
		return
	}
	total, err := h.queries.CountPublishedPhotoAlbums(r.Context())
	if err != nil {
		writePublicPhotoError(w, http.StatusInternalServerError, "相册列表读取失败")
		return
	}
	rows := []dbgen.ListPublishedPhotoAlbumsRow{}
	if offset, inRange := pageOffset(pageParams.Page, pageParams.Limit); inRange {
		rows, err = h.queries.ListPublishedPhotoAlbums(r.Context(), dbgen.ListPublishedPhotoAlbumsParams{
			Limit:  int32(pageParams.Limit),
			Offset: int32(offset),
		})
		if err != nil {
			writePublicPhotoError(w, http.StatusInternalServerError, "相册列表读取失败")
			return
		}
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapPublicAlbum(publicAlbumFields{
			Slug:          row.Slug,
			Title:         row.Title,
			TitleEn:       row.TitleEn,
			Description:   row.Description,
			DescriptionEn: row.DescriptionEn,
			Tags:          row.Tags,
			PublishedAt:   row.PublishedAt,
			PhotoCount:    row.PhotoCount,
			CoverUrl:      row.CoverUrl,
			CoverWidth:    row.CoverWidth,
			CoverHeight:   row.CoverHeight,
		}))
	}
	writePublicPhotoJSON(w, r, pageEnvelope(items, pageParams.Page, pageParams.Limit, int(total)))
}

func (h Handler) getPublishedAlbum(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !ValidAlbumSlug(slug) {
		writePublicPhotoError(w, http.StatusNotFound, "相册不存在")
		return
	}
	row, err := h.queries.GetPublishedPhotoAlbumBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writePublicPhotoError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writePublicPhotoError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	writePublicPhotoJSON(w, r, mapPublicAlbum(publicAlbumFields{
		Slug:          row.Slug,
		Title:         row.Title,
		TitleEn:       row.TitleEn,
		Description:   row.Description,
		DescriptionEn: row.DescriptionEn,
		Tags:          row.Tags,
		PublishedAt:   row.PublishedAt,
		PhotoCount:    row.PhotoCount,
		CoverUrl:      row.CoverUrl,
		CoverWidth:    row.CoverWidth,
		CoverHeight:   row.CoverHeight,
	}))
}

func (h Handler) listPublishedAlbumPhotos(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !ValidAlbumSlug(slug) {
		writePublicPhotoError(w, http.StatusNotFound, "相册不存在")
		return
	}
	if _, err := h.queries.GetPublishedPhotoAlbumBySlug(r.Context(), slug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writePublicPhotoError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writePublicPhotoError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	pageParams, ok := parsePagination(r, defaultPhotoPageLimit)
	if !ok {
		writePublicPhotoError(w, http.StatusBadRequest, "分页参数不正确")
		return
	}
	total, err := h.queries.CountPublishedAlbumPhotosBySlug(r.Context(), slug)
	if err != nil {
		writePublicPhotoError(w, http.StatusInternalServerError, "相册照片读取失败")
		return
	}
	rows := []dbgen.ListPublishedAlbumPhotosBySlugRow{}
	if offset, inRange := pageOffset(pageParams.Page, pageParams.Limit); inRange {
		rows, err = h.queries.ListPublishedAlbumPhotosBySlug(r.Context(), dbgen.ListPublishedAlbumPhotosBySlugParams{
			Slug:   slug,
			Limit:  int32(pageParams.Limit),
			Offset: int32(offset),
		})
		if err != nil {
			writePublicPhotoError(w, http.StatusInternalServerError, "相册照片读取失败")
			return
		}
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapPublicPhoto(publicPhotoFields{
			Url:     row.Url,
			Alt:     row.Alt,
			Caption: row.Caption,
			Width:   row.Width,
			Height:  row.Height,
			TakenAt: row.TakenAt,
		}))
	}
	writePublicPhotoJSON(w, r, pageEnvelope(items, pageParams.Page, pageParams.Limit, int(total)))
}

// ---- 分页与映射 ----

type pageParams struct {
	Page  int
	Limit int
}

// parsePagination 约定：page/limit 缺省用默认值；非整数或小于 1 返回 false（400）；
// limit 超过 maxPageLimit 时按上限处理。
func parsePagination(r *http.Request, defaultLimit int) (pageParams, bool) {
	pageQuery := strings.TrimSpace(r.URL.Query().Get("page"))
	limitQuery := strings.TrimSpace(r.URL.Query().Get("limit"))
	page := 1
	limit := defaultLimit
	if pageQuery != "" {
		value, err := strconv.Atoi(pageQuery)
		if err != nil || value < 1 {
			return pageParams{}, false
		}
		page = value
	}
	if limitQuery != "" {
		value, err := strconv.Atoi(limitQuery)
		if err != nil || value < 1 {
			return pageParams{}, false
		}
		limit = value
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	return pageParams{Page: page, Limit: limit}, true
}

// pageOffset 计算偏移并拦截整型溢出；页码超出 int32 偏移范围时视为越界页（返回空数据）。
func pageOffset(page int, limit int) (int, bool) {
	offset := (page - 1) * limit
	if page < 1 || limit < 1 || offset > math.MaxInt32 {
		return 0, false
	}
	return offset, true
}

func pageEnvelope(data interface{}, page int, limit int, total int) map[string]interface{} {
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return map[string]interface{}{
		"data":       data,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	}
}

type publicAlbumFields struct {
	Slug          string
	Title         string
	TitleEn       pgtype.Text
	Description   string
	DescriptionEn pgtype.Text
	Tags          []string
	PublishedAt   pgtype.Timestamptz
	PhotoCount    int32
	CoverUrl      pgtype.Text
	CoverWidth    int32
	CoverHeight   int32
}

// mapPublicAlbum 输出公开契约字段；封面缺失（相册照片被删空）时宽高为 null。
func mapPublicAlbum(fields publicAlbumFields) map[string]interface{} {
	tags := fields.Tags
	if tags == nil {
		tags = []string{}
	}
	return map[string]interface{}{
		"slug":          fields.Slug,
		"title":         fields.Title,
		"titleEn":       textValue(fields.TitleEn),
		"description":   fields.Description,
		"descriptionEn": textValue(fields.DescriptionEn),
		"coverUrl":      textValue(fields.CoverUrl),
		"coverWidth":    positiveInt32Value(fields.CoverWidth),
		"coverHeight":   positiveInt32Value(fields.CoverHeight),
		"publishedAt":   timestamptzValue(fields.PublishedAt),
		"tags":          tags,
		"photoCount":    fields.PhotoCount,
	}
}

type publicPhotoFields struct {
	Url     pgtype.Text
	Alt     string
	Caption pgtype.Text
	Width   int32
	Height  int32
	TakenAt pgtype.Timestamptz
}

func mapPublicPhoto(fields publicPhotoFields) map[string]interface{} {
	return map[string]interface{}{
		"url":     textValue(fields.Url),
		"alt":     fields.Alt,
		"caption": textValue(fields.Caption),
		"width":   fields.Width,
		"height":  fields.Height,
		"takenAt": timestamptzValue(fields.TakenAt),
	}
}

// ---- 通用助手 ----

func cleanStoragePrefix(value string, fallback string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" || value == "." {
		return fallback
	}
	return value
}

func uuidFromString(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func textValue(value pgtype.Text) interface{} {
	if !value.Valid {
		return nil
	}
	return value.String
}

func positiveInt32Value(value int32) interface{} {
	if value <= 0 {
		return nil
	}
	return value
}

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func isUniqueViolation(err error, constraintPrefix string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && strings.HasPrefix(pgErr.ConstraintName, constraintPrefix)
}

func requireUser(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := httpapi.UserFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writePublicPhotoJSON 与文章公开接口一致：ETag + 60 秒缓存 + 304 协商。
func writePublicPhotoJSON(w http.ResponseWriter, r *http.Request, body interface{}) {
	payload, err := json.Marshal(body)
	if err != nil {
		writePublicPhotoError(w, http.StatusInternalServerError, "响应序列化失败")
		return
	}

	sum := sha256.Sum256(payload)
	etag := fmt.Sprintf(`"%x"`, sum)
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Set("ETag", etag)
	if requestETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(payload, '\n'))
}

func requestETagMatches(header string, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

func writePublicPhotoError(w http.ResponseWriter, status int, message string) {
	if status == http.StatusNotFound {
		w.Header().Set("Cache-Control", "public, max-age=15")
	} else if status >= http.StatusInternalServerError {
		w.Header().Set("Cache-Control", "no-store")
	}
	writeJSON(w, status, map[string]string{"error": message})
}
