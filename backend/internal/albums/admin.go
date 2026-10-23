package albums

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

type AlbumPayload struct {
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	TitleEn       string   `json:"titleEn"`
	Description   string   `json:"description"`
	DescriptionEn string   `json:"descriptionEn"`
	Tags          []string `json:"tags"`
}

type AlbumPhotoPayload struct {
	Alt     string  `json:"alt"`
	Caption string  `json:"caption"`
	TakenAt *string `json:"takenAt"`
}

type AlbumPhotoOrderPayload struct {
	PhotoIds []string `json:"photoIds"`
}

type AlbumCoverPayload struct {
	PhotoID *string `json:"photoId"`
}

func (h Handler) listAdminAlbums(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	rows, err := h.queries.ListAdminPhotoAlbums(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "后台相册列表读取失败")
		return
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAdminAlbum(adminAlbumFields{
			AlbumID:       row.AlbumID,
			Slug:          row.Slug,
			Title:         row.Title,
			TitleEn:       row.TitleEn,
			Description:   row.Description,
			DescriptionEn: row.DescriptionEn,
			Status:        row.Status,
			Tags:          row.Tags,
			CoverPhotoID:  row.CoverPhotoID,
			PublishedAt:   row.PublishedAt,
			CreatedBy:     row.CreatedBy,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			PhotoCount:    row.PhotoCount,
			CoverUrl:      row.CoverUrl,
			CoverWidth:    row.CoverWidth,
			CoverHeight:   row.CoverHeight,
		}))
	}
	writeJSON(w, http.StatusOK, items)
}

func (h Handler) getAdminAlbum(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	row, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapAdminAlbum(adminAlbumFieldsFromRow(row)))
}

func (h Handler) createAlbum(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}
	payload, ok := decodeAlbumPayload(w, r)
	if !ok {
		return
	}
	album, err := h.queries.CreatePhotoAlbum(r.Context(), dbgen.CreatePhotoAlbumParams{
		Slug:          payload.Slug,
		Title:         payload.Title,
		TitleEn:       nullableText(payload.TitleEn),
		Description:   payload.Description,
		DescriptionEn: nullableText(payload.DescriptionEn),
		Tags:          payload.Tags,
		CreatedBy:     user,
	})
	if err != nil {
		if isUniqueViolation(err, "photo_albums_slug") {
			writeError(w, http.StatusBadRequest, "相册 slug 已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册创建失败")
		return
	}
	writeJSON(w, http.StatusCreated, mapAdminAlbumFromModel(album))
}

func (h Handler) updateAlbum(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	payload, ok := decodeAlbumPayload(w, r)
	if !ok {
		return
	}
	current, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	// slug 是 Rendering 路由 /photos/{slug} 的稳定标识，发布后不允许修改。
	if current.Status == dbgen.PhotoAlbumStatusPublished && payload.Slug != current.Slug {
		writeError(w, http.StatusConflict, "已发布相册的 slug 不可修改")
		return
	}
	album, err := h.queries.UpdatePhotoAlbum(r.Context(), dbgen.UpdatePhotoAlbumParams{
		AlbumID:       albumID,
		Slug:          payload.Slug,
		Title:         payload.Title,
		TitleEn:       nullableText(payload.TitleEn),
		Description:   payload.Description,
		DescriptionEn: nullableText(payload.DescriptionEn),
		Tags:          payload.Tags,
	})
	if err != nil {
		if isUniqueViolation(err, "photo_albums_slug") {
			writeError(w, http.StatusBadRequest, "相册 slug 已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册更新失败")
		return
	}
	// 以联查行返回完整信息（照片数、封面）。
	h.writeAdminAlbumByID(w, r, album.AlbumID)
}

func (h Handler) publishAlbum(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	current, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	if current.Status == dbgen.PhotoAlbumStatusPublished {
		writeError(w, http.StatusConflict, "相册已发布")
		return
	}
	count, err := h.queries.CountAlbumPhotosByID(r.Context(), albumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "相册照片读取失败")
		return
	}
	if count < 1 {
		writeError(w, http.StatusConflict, "相册至少需要一张照片才能发布")
		return
	}
	if _, err := h.queries.PublishPhotoAlbum(r.Context(), albumID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "相册已发布")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册发布失败")
		return
	}
	h.writeAdminAlbumByID(w, r, albumID)
}

func (h Handler) unpublishAlbum(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	current, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	if current.Status != dbgen.PhotoAlbumStatusPublished {
		writeError(w, http.StatusConflict, "相册未发布")
		return
	}
	if _, err := h.queries.UnpublishPhotoAlbum(r.Context(), albumID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "相册未发布")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册下架失败")
		return
	}
	h.writeAdminAlbumByID(w, r, albumID)
}

func (h Handler) deleteAlbum(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	if _, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	keys, err := h.queries.ListAlbumAssetKeys(r.Context(), albumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "相册照片读取失败")
		return
	}
	if _, err := h.queries.SoftDeleteAlbumAssets(r.Context(), albumID); err != nil {
		writeError(w, http.StatusInternalServerError, "相册资源清理失败")
		return
	}
	if err := h.queries.DeletePhotoAlbum(r.Context(), albumID); err != nil {
		writeError(w, http.StatusInternalServerError, "相册删除失败")
		return
	}
	// 对象存储文件尽力删除；失败不影响相册删除结果，残留对象由存储生命周期兜底。
	h.deleteObjectsBestEffort(r, keys)
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) listAdminAlbumPhotos(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	rows, err := h.queries.ListAlbumPhotos(r.Context(), albumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "相册照片读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapAdminPhotos(rows))
}

func (h Handler) updateAlbumPhoto(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, photoID, ok := albumPhotoIDsFromRequest(w, r)
	if !ok {
		return
	}
	var payload AlbumPhotoPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	if err := ValidatePhotoAlt(payload.Alt); err != nil {
		writeError(w, http.StatusBadRequest, "照片 alt 不能为空且不能超过 500 个字符")
		return
	}
	if err := ValidatePhotoCaption(payload.Caption); err != nil {
		writeError(w, http.StatusBadRequest, "照片说明不能超过 1000 个字符")
		return
	}
	takenAt, ok := parseTakenAt(w, payload.TakenAt)
	if !ok {
		return
	}
	if _, err := h.queries.GetAlbumPhotoByID(r.Context(), dbgen.GetAlbumPhotoByIDParams{AlbumID: albumID, PhotoID: photoID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "照片不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "照片读取失败")
		return
	}
	if _, err := h.queries.UpdateAlbumPhoto(r.Context(), dbgen.UpdateAlbumPhotoParams{
		AlbumID: albumID,
		PhotoID: photoID,
		Alt:     payload.Alt,
		Caption: nullableText(payload.Caption),
		TakenAt: takenAt,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "照片不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "照片更新失败")
		return
	}
	row, err := h.queries.GetAlbumPhotoByID(r.Context(), dbgen.GetAlbumPhotoByIDParams{AlbumID: albumID, PhotoID: photoID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "照片读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapAdminPhoto(row))
}

func (h Handler) deleteAlbumPhoto(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, photoID, ok := albumPhotoIDsFromRequest(w, r)
	if !ok {
		return
	}
	photo, err := h.queries.GetAlbumPhotoByID(r.Context(), dbgen.GetAlbumPhotoByIDParams{AlbumID: albumID, PhotoID: photoID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "照片不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "照片读取失败")
		return
	}
	deleted, err := h.queries.DeleteAlbumPhoto(r.Context(), dbgen.DeleteAlbumPhotoParams{AlbumID: albumID, PhotoID: photoID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "照片删除失败")
		return
	}
	if deleted == 0 {
		writeError(w, http.StatusNotFound, "照片不存在")
		return
	}
	if err := h.queries.ClearPhotoAlbumCoverIfRemoved(r.Context(), dbgen.ClearPhotoAlbumCoverIfRemovedParams{
		AlbumID:      albumID,
		CoverPhotoID: photoID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "相册封面更新失败")
		return
	}
	if _, err := h.queries.UpdateAssetStatus(r.Context(), dbgen.UpdateAssetStatusParams{
		AssetID:   photo.AssetID,
		Status:    dbgen.AssetStatusDeleted,
		DeletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "照片资源清理失败")
		return
	}
	h.deleteObjectBestEffort(r, photo.StorageKey)
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) reorderAlbumPhotos(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	var payload AlbumPhotoOrderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	existing, err := h.queries.ListAlbumPhotos(r.Context(), albumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "相册照片读取失败")
		return
	}
	if len(payload.PhotoIds) != len(existing) {
		writeError(w, http.StatusBadRequest, "排序数组必须包含相册内全部照片")
		return
	}
	known := make(map[string]bool, len(existing))
	for _, photo := range existing {
		known[photo.PhotoID.String()] = true
	}
	photoIDs := make([]pgtype.UUID, 0, len(payload.PhotoIds))
	positions := make([]int32, 0, len(payload.PhotoIds))
	for index, rawID := range payload.PhotoIds {
		id, err := uuidFromString(rawID)
		if err != nil || !known[rawID] {
			writeError(w, http.StatusBadRequest, "排序数组必须包含相册内全部照片且不重复")
			return
		}
		delete(known, rawID)
		photoIDs = append(photoIDs, id)
		positions = append(positions, int32(index+1))
	}
	if err := h.queries.ReorderAlbumPhotos(r.Context(), dbgen.ReorderAlbumPhotosParams{
		PhotoIds:  photoIDs,
		Positions: positions,
		AlbumID:   albumID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "照片排序失败")
		return
	}
	rows, err := h.queries.ListAlbumPhotos(r.Context(), albumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "相册照片读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapAdminPhotos(rows))
}

func (h Handler) setAlbumCover(w http.ResponseWriter, r *http.Request) {
	if !requireUser(w, r) {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	var payload AlbumCoverPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	if _, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	coverPhotoID := pgtype.UUID{}
	if payload.PhotoID != nil && *payload.PhotoID != "" {
		photoID, err := uuidFromString(*payload.PhotoID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "照片 ID 无效")
			return
		}
		if _, err := h.queries.GetAlbumPhotoByID(r.Context(), dbgen.GetAlbumPhotoByIDParams{AlbumID: albumID, PhotoID: photoID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "封面照片不存在于该相册")
				return
			}
			writeError(w, http.StatusInternalServerError, "照片读取失败")
			return
		}
		coverPhotoID = photoID
	}
	if _, err := h.queries.SetPhotoAlbumCover(r.Context(), dbgen.SetPhotoAlbumCoverParams{
		AlbumID:      albumID,
		CoverPhotoID: coverPhotoID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "相册封面更新失败")
		return
	}
	h.writeAdminAlbumByID(w, r, albumID)
}

// ---- 后台映射与助手 ----

type adminAlbumFields struct {
	AlbumID       pgtype.UUID
	Slug          string
	Title         string
	TitleEn       pgtype.Text
	Description   string
	DescriptionEn pgtype.Text
	Status        dbgen.PhotoAlbumStatus
	Tags          []string
	CoverPhotoID  pgtype.UUID
	PublishedAt   pgtype.Timestamptz
	CreatedBy     pgtype.UUID
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
	PhotoCount    int32
	CoverUrl      pgtype.Text
	CoverWidth    int32
	CoverHeight   int32
}

func mapAdminAlbum(fields adminAlbumFields) map[string]interface{} {
	tags := fields.Tags
	if tags == nil {
		tags = []string{}
	}
	coverPhotoID := interface{}(nil)
	if fields.CoverPhotoID.Valid {
		coverPhotoID = fields.CoverPhotoID.String()
	}
	return map[string]interface{}{
		"albumId":       fields.AlbumID.String(),
		"slug":          fields.Slug,
		"title":         fields.Title,
		"titleEn":       textValue(fields.TitleEn),
		"description":   fields.Description,
		"descriptionEn": textValue(fields.DescriptionEn),
		"status":        string(fields.Status),
		"tags":          tags,
		"coverPhotoId":  coverPhotoID,
		"publishedAt":   timestamptzValue(fields.PublishedAt),
		"createdBy":     fields.CreatedBy.String(),
		"createdAt":     fields.CreatedAt.Time,
		"updatedAt":     fields.UpdatedAt.Time,
		"photoCount":    fields.PhotoCount,
		"coverUrl":      textValue(fields.CoverUrl),
		"coverWidth":    positiveInt32Value(fields.CoverWidth),
		"coverHeight":   positiveInt32Value(fields.CoverHeight),
	}
}

// mapAdminAlbumFromModel 用于创建相册的响应：新相册没有照片与封面，零值即真实值。
func mapAdminAlbumFromModel(album dbgen.PhotoAlbum) map[string]interface{} {
	return mapAdminAlbum(adminAlbumFields{
		AlbumID:       album.AlbumID,
		Slug:          album.Slug,
		Title:         album.Title,
		TitleEn:       album.TitleEn,
		Description:   album.Description,
		DescriptionEn: album.DescriptionEn,
		Status:        album.Status,
		Tags:          album.Tags,
		CoverPhotoID:  album.CoverPhotoID,
		PublishedAt:   album.PublishedAt,
		CreatedBy:     album.CreatedBy,
		CreatedAt:     album.CreatedAt,
		UpdatedAt:     album.UpdatedAt,
		PhotoCount:    0,
		CoverUrl:      pgtype.Text{},
		CoverWidth:    0,
		CoverHeight:   0,
	})
}

func adminAlbumFieldsFromRow(row dbgen.GetPhotoAlbumByIDRow) adminAlbumFields {
	return adminAlbumFields{
		AlbumID:       row.AlbumID,
		Slug:          row.Slug,
		Title:         row.Title,
		TitleEn:       row.TitleEn,
		Description:   row.Description,
		DescriptionEn: row.DescriptionEn,
		Status:        row.Status,
		Tags:          row.Tags,
		CoverPhotoID:  row.CoverPhotoID,
		PublishedAt:   row.PublishedAt,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		PhotoCount:    row.PhotoCount,
		CoverUrl:      row.CoverUrl,
		CoverWidth:    row.CoverWidth,
		CoverHeight:   row.CoverHeight,
	}
}

// writeAdminAlbumByID 以完整联查行（照片数、封面）写回后台相册响应。
func (h Handler) writeAdminAlbumByID(w http.ResponseWriter, r *http.Request, albumID pgtype.UUID) {
	row, err := h.queries.GetPhotoAlbumByID(r.Context(), albumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapAdminAlbum(adminAlbumFieldsFromRow(row)))
}

func mapAdminPhotos(rows []dbgen.ListAlbumPhotosRow) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]interface{}{
			"photoId":     row.PhotoID.String(),
			"alt":         row.Alt,
			"caption":     textValue(row.Caption),
			"width":       row.Width,
			"height":      row.Height,
			"takenAt":     timestamptzValue(row.TakenAt),
			"position":    row.Position,
			"createdAt":   row.CreatedAt.Time,
			"url":         textValue(row.PublicUrl),
			"filename":    row.Filename,
			"contentType": row.ContentType,
			"byteSize":    row.ByteSize,
		})
	}
	return items
}

func mapAdminPhoto(row dbgen.GetAlbumPhotoByIDRow) map[string]interface{} {
	return map[string]interface{}{
		"photoId":     row.PhotoID.String(),
		"alt":         row.Alt,
		"caption":     textValue(row.Caption),
		"width":       row.Width,
		"height":      row.Height,
		"takenAt":     timestamptzValue(row.TakenAt),
		"position":    row.Position,
		"createdAt":   row.CreatedAt.Time,
		"url":         textValue(row.PublicUrl),
		"filename":    row.Filename,
		"contentType": row.ContentType,
		"byteSize":    row.ByteSize,
	}
}

func decodeAlbumPayload(w http.ResponseWriter, r *http.Request) (AlbumPayload, bool) {
	var payload AlbumPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return payload, false
	}
	payload.Slug = strings.TrimSpace(payload.Slug)
	payload.Title = strings.TrimSpace(payload.Title)
	payload.TitleEn = strings.TrimSpace(payload.TitleEn)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.DescriptionEn = strings.TrimSpace(payload.DescriptionEn)
	if payload.Tags == nil {
		payload.Tags = []string{}
	}
	if err := ValidateAlbumInput(payload.Slug, payload.Title, payload.Description); err != nil {
		writeError(w, http.StatusBadRequest, "相册 slug、标题或导语不合法")
		return payload, false
	}
	if err := ValidateOptionalText(payload.TitleEn, maxAlbumTitleRunes); err != nil {
		writeError(w, http.StatusBadRequest, "英文标题不能超过 200 个字符")
		return payload, false
	}
	if err := ValidateOptionalText(payload.DescriptionEn, maxAlbumDescriptionRunes); err != nil {
		writeError(w, http.StatusBadRequest, "英文导语不能超过 2000 个字符")
		return payload, false
	}
	if err := ValidateTagList(payload.Tags); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return payload, false
	}
	return payload, true
}

func userFromRequest(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	user, ok := httpUserFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return pgtype.UUID{}, false
	}
	userID, err := uuidFromString(user.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "用户 ID 无效")
		return pgtype.UUID{}, false
	}
	return userID, true
}

func albumIDFromRequest(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	albumID, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "相册 ID 无效")
		return pgtype.UUID{}, false
	}
	return albumID, true
}

func albumPhotoIDsFromRequest(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	photoID, err := uuidFromString(chi.URLParam(r, "photoId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "照片 ID 无效")
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return albumID, photoID, true
}

func parseTakenAt(w http.ResponseWriter, raw *string) (pgtype.Timestamptz, bool) {
	if raw == nil || *raw == "" {
		return pgtype.Timestamptz{}, true
	}
	taken, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "拍摄时间格式不正确")
		return pgtype.Timestamptz{}, false
	}
	return pgtype.Timestamptz{Time: taken, Valid: true}, true
}
