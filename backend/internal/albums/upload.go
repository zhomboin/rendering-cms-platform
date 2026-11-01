package albums

import (
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
	"rendering-cms-platform/backend/internal/imaging"
	"rendering-cms-platform/backend/internal/uploadpolicy"
)

// uploadAlbumPhoto 是后端代理上传：multipart 直传后端，服务端完成魔数校验、
// 尺寸提取、EXIF/XMP 元数据剥离与拍摄时间提取后写入对象存储与数据库。
// 照片天然公开可读，未配置公开访问基地址时直接拒绝上传。
func (h Handler) uploadAlbumPhoto(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}
	albumID, ok := albumIDFromRequest(w, r)
	if !ok {
		return
	}
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "对象存储未配置")
		return
	}
	if h.options.PublicBaseURL == "" {
		writeError(w, http.StatusServiceUnavailable, "照片上传未配置公开访问基地址（S3_PUBLIC_BASE_URL）")
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

	// 即使独立注册 handler，也限制分块请求并回收解析产生的临时文件。
	if r.ContentLength > uploadpolicy.PhotoRequestBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, uploadpolicy.PhotoRequestBytes)
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			writeError(w, http.StatusRequestEntityTooLarge, "请求体过大")
		} else {
			writeError(w, http.StatusBadRequest, "照片表单无效")
		}
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少照片文件")
		return
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxPhotoUploadBytes {
		writeError(w, http.StatusBadRequest, "图片大小必须大于 0 且不超过 50MB")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "照片文件读取失败")
		return
	}
	if !ValidPhotoUploadSize(len(data)) {
		writeError(w, http.StatusBadRequest, "图片大小必须大于 0 且不超过 50MB")
		return
	}

	contentType, ok := imaging.Sniff(data)
	if !ok || !ValidPhotoContentType(contentType) {
		writeError(w, http.StatusBadRequest, "照片只支持 PNG、JPEG 或 WebP 格式")
		return
	}
	width, height, err := imaging.Dimensions(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "照片数据损坏，无法解析尺寸")
		return
	}

	alt := r.FormValue("alt")
	if err := ValidatePhotoAlt(alt); err != nil {
		writeError(w, http.StatusBadRequest, "照片 alt 不能为空且不能超过 500 个字符")
		return
	}
	caption := r.FormValue("caption")
	if err := ValidatePhotoCaption(caption); err != nil {
		writeError(w, http.StatusBadRequest, "照片说明不能超过 1000 个字符")
		return
	}
	takenAt := pgtype.Timestamptz{}
	if raw := r.FormValue("takenAt"); raw != "" {
		value, ok := parseTakenAt(w, &raw)
		if !ok {
			return
		}
		takenAt = value
	} else if exifTaken, found := imaging.TakenAt(data, contentType); found {
		// EXIF 拍摄时间仅在表单未显式提供时作为默认值。
		takenAt = pgtype.Timestamptz{Time: exifTaken, Valid: true}
	}

	// 存储前剥离 EXIF/XMP 等元数据（含 GPS 等隐私信息），不重编码像素。
	cleaned, _, err := imaging.StripMetadata(data, contentType)
	if err != nil {
		writeError(w, http.StatusBadRequest, "照片处理失败")
		return
	}

	storageKey := h.photoStorageKey(contentType)
	if err := h.store.PutObject(r.Context(), storageKey, cleaned, contentType, photoObjectCacheControl); err != nil {
		writeError(w, http.StatusInternalServerError, "照片存储写入失败")
		return
	}

	asset, err := h.queries.CreateAlbumPhotoAsset(r.Context(), dbgen.CreateAlbumPhotoAssetParams{
		Filename:    photoFilename(header.Filename, contentType),
		ContentType: contentType,
		ByteSize:    int32(len(cleaned)),
		StorageKey:  storageKey,
		PublicUrl:   nullableText(h.options.PublicBaseURL + "/" + storageKey),
		CreatedBy:   user,
		Width:       pgtype.Int4{Int32: int32(width), Valid: true},
		Height:      pgtype.Int4{Int32: int32(height), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "照片资源记录创建失败")
		return
	}
	photo, err := h.queries.CreateAlbumPhoto(r.Context(), dbgen.CreateAlbumPhotoParams{
		AlbumID: albumID,
		AssetID: asset.AssetID,
		Alt:     alt,
		Caption: nullableText(caption),
		Width:   int32(width),
		Height:  int32(height),
		TakenAt: takenAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "照片记录创建失败")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"photoId":     photo.PhotoID.String(),
		"alt":         photo.Alt,
		"caption":     textValue(photo.Caption),
		"width":       photo.Width,
		"height":      photo.Height,
		"takenAt":     timestamptzValue(photo.TakenAt),
		"position":    photo.Position,
		"createdAt":   photo.CreatedAt.Time,
		"url":         textValue(asset.PublicUrl),
		"filename":    asset.Filename,
		"contentType": asset.ContentType,
		"byteSize":    asset.ByteSize,
	})
}

// photoStorageKey 生成不可变对象 key：photos/{年}/{月}/{uuid}.{权威扩展名}。
func (h Handler) photoStorageKey(contentType string) string {
	now := h.options.Now().UTC()
	name := uuid.NewString() + ExtensionForContentType(contentType)
	return path.Join(h.options.PhotoPrefix, now.Format("2006"), now.Format("01"), name)
}

// photoFilename 保留原始文件名用于后台展示；缺失时按权威类型生成兜底名。
func photoFilename(filename string, contentType string) string {
	if filename != "" {
		return path.Base(strings.ReplaceAll(filename, "\\", "/"))
	}
	return "photo" + ExtensionForContentType(contentType)
}

// deleteObjectBestEffort 删除对象存储文件；失败仅忽略，不影响主流程。
func (h Handler) deleteObjectBestEffort(r *http.Request, storageKey string) {
	if h.store == nil || storageKey == "" {
		return
	}
	_ = h.store.DeleteObject(r.Context(), storageKey)
}

func (h Handler) deleteObjectsBestEffort(r *http.Request, keys []dbgen.ListAlbumAssetKeysRow) {
	for _, key := range keys {
		h.deleteObjectBestEffort(r, key.StorageKey)
	}
}

func httpUserFromContext(r *http.Request) (httpapi.AuthenticatedUser, bool) {
	return httpapi.UserFromContext(r.Context())
}
