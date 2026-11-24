package albums

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

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

	in, ok := ReadPhotoUpload(w, r)
	if !ok {
		return
	}
	staged, err := h.StagePhoto(r.Context(), in)
	if err != nil {
		var invalid PhotoInputError
		if errors.As(err, &invalid) {
			writeError(w, 400, err.Error())
		} else {
			writeError(w, 500, "照片上传失败")
		}
		return
	}
	var saved SavedPhoto
	err = h.queries.WithinAlbumWrite(r.Context(), albumID, func(ctx context.Context, q albumStore) error {
		var err error
		saved, err = staged.Attach(ctx, q, albumID, user)
		return err
	})
	if err != nil {
		// 重新认领持久任务，避免提交结果未知时误删已经关联的照片。
		staged.Cleanup(r.Context())
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
		} else {
			writeError(w, http.StatusInternalServerError, "照片记录创建失败")
		}
		return
	}

	photo, asset := saved.Photo, saved.Asset
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

type PhotoUpload struct {
	Data                            []byte
	Filename, Alt, Caption, TakenAt string
}

func ReadPhotoUpload(w http.ResponseWriter, r *http.Request) (PhotoUpload, bool) {
	// 即使独立注册 handler，也限制分块请求并回收解析产生的临时文件。
	if r.ContentLength > uploadpolicy.PhotoRequestBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "请求体过大")
		return PhotoUpload{}, false
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
		return PhotoUpload{}, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少照片文件")
		return PhotoUpload{}, false
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxPhotoUploadBytes {
		writeError(w, http.StatusBadRequest, "图片大小必须大于 0 且不超过 50MB")
		return PhotoUpload{}, false
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "照片文件读取失败")
		return PhotoUpload{}, false
	}
	if !ValidPhotoUploadSize(len(data)) {
		writeError(w, http.StatusBadRequest, "图片大小必须大于 0 且不超过 50MB")
		return PhotoUpload{}, false
	}

	return PhotoUpload{Data: data, Filename: header.Filename, Alt: r.FormValue("alt"), Caption: r.FormValue("caption"), TakenAt: r.FormValue("takenAt")}, true
}

type PhotoInputError string

func (e PhotoInputError) Error() string { return string(e) }

type SavedPhoto struct {
	Photo dbgen.AlbumPhoto
	Asset dbgen.Asset
}
type photoAttachmentStore interface {
	CreateAlbumPhotoAsset(context.Context, dbgen.CreateAlbumPhotoAssetParams) (dbgen.Asset, error)
	CreateAlbumPhoto(context.Context, dbgen.CreateAlbumPhotoParams) (dbgen.AlbumPhoto, error)
	CancelAlbumObjectCleanup(context.Context, string) (int64, error)
}
type StagedPhoto struct {
	handler Handler
	asset   dbgen.CreateAlbumPhotoAssetParams
	photo   dbgen.CreateAlbumPhotoParams
}

func (p *StagedPhoto) Cleanup(ctx context.Context) {
	p.handler.CleanupObjectsAfterWrite(ctx, p.asset.StorageKey)
}

func (p *StagedPhoto) Attach(ctx context.Context, q photoAttachmentStore, albumID, owner pgtype.UUID) (SavedPhoto, error) {
	assetInput := p.asset
	assetInput.CreatedBy = owner
	asset, err := q.CreateAlbumPhotoAsset(ctx, assetInput)
	if err != nil {
		return SavedPhoto{}, err
	}
	photoInput := p.photo
	photoInput.AlbumID, photoInput.AssetID = albumID, asset.AssetID
	photo, err := q.CreateAlbumPhoto(ctx, photoInput)
	if err != nil {
		return SavedPhoto{}, err
	}
	cancelled, err := q.CancelAlbumObjectCleanup(ctx, p.asset.StorageKey)
	if err != nil {
		return SavedPhoto{}, err
	}
	if cancelled != 1 {
		return SavedPhoto{}, errors.New("照片清理任务已被接管")
	}
	return SavedPhoto{Photo: photo, Asset: asset}, nil
}

// StagePhoto 先登记持久清理任务，再写入去除隐私元数据的照片。调用方在业务事务内关联照片。
func (h Handler) StagePhoto(ctx context.Context, in PhotoUpload) (*StagedPhoto, error) {
	if h.store == nil || h.options.PublicBaseURL == "" {
		return nil, errors.New("相册对象存储未配置")
	}
	if !ValidPhotoUploadSize(len(in.Data)) {
		return nil, PhotoInputError("图片大小必须大于 0 且不超过 50MB")
	}
	contentType, ok := imaging.Sniff(in.Data)
	if !ok || !ValidPhotoContentType(contentType) {
		return nil, PhotoInputError("照片只支持 PNG、JPEG 或 WebP 格式")
	}
	width, height, err := imaging.Dimensions(in.Data)
	if err != nil {
		return nil, PhotoInputError("照片数据损坏，无法解析尺寸")
	}

	alt := in.Alt
	if err := ValidatePhotoAlt(alt); err != nil {
		return nil, PhotoInputError("照片 alt 不能为空且不能超过 500 个字符")
	}
	caption := in.Caption
	if err := ValidatePhotoCaption(caption); err != nil {
		return nil, PhotoInputError("照片说明不能超过 1000 个字符")
	}
	takenAt := pgtype.Timestamptz{}
	if raw := in.TakenAt; raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, PhotoInputError("拍摄时间格式不正确")
		}
		takenAt = pgtype.Timestamptz{Time: value, Valid: true}
	} else if exifTaken, found := imaging.TakenAt(in.Data, contentType); found {
		// EXIF 拍摄时间仅在表单未显式提供时作为默认值。
		takenAt = pgtype.Timestamptz{
			Time:  exifTaken,
			Valid: true,
		}
	}

	// 存储前剥离 EXIF/XMP 等元数据（含 GPS 等隐私信息），不重编码像素。
	cleaned, _, err := imaging.StripMetadata(in.Data, contentType)
	if err != nil {
		return nil, PhotoInputError("照片处理失败")
	}

	storageKey := h.photoStorageKey(contentType)
	uploadCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := h.queries.EnqueueAlbumObjectCleanup(uploadCtx, dbgen.EnqueueAlbumObjectCleanupParams{
		StorageKey:    storageKey,
		NextAttemptAt: pgtype.Timestamptz{Time: h.options.Now().Add(15 * time.Minute), Valid: true},
	}); err != nil {
		return nil, errors.New("照片上传登记失败")
	}
	if err := h.store.PutObject(uploadCtx, storageKey, cleaned, contentType, photoObjectCacheControl); err != nil {
		// 存储结果可能未知，保留清理意图，等请求期限过去后由后台重试。
		return nil, errors.New("照片存储写入失败")
	}
	return &StagedPhoto{
		handler: h,
		asset: dbgen.CreateAlbumPhotoAssetParams{
			Filename: photoFilename(in.Filename, contentType), ContentType: contentType, ByteSize: int32(len(cleaned)),
			StorageKey: storageKey, PublicUrl: nullableText(h.options.PublicBaseURL + "/" + storageKey),
			Width: pgtype.Int4{Int32: int32(width), Valid: true}, Height: pgtype.Int4{Int32: int32(height), Valid: true},
		},
		photo: dbgen.CreateAlbumPhotoParams{
			Alt: alt, Caption: nullableText(caption), TakenAt: takenAt, Width: int32(width), Height: int32(height),
		},
	}, nil
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

func httpUserFromContext(r *http.Request) (httpapi.AuthenticatedUser, bool) {
	return httpapi.UserFromContext(r.Context())
}
