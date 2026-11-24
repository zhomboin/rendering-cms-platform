package articlemcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"rendering-cms-platform/backend/internal/albums"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

const uploadRoute = endpoint + "/albums/{id}/photos"
const inlinePhotoLimit = 1 << 20

type PhotoUploadInput struct {
	AlbumRef
	RequestKey string `json:"requestKey"`
	DataBase64 string `json:"dataBase64,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Alt        string `json:"alt,omitempty"`
	Caption    string `json:"caption,omitempty"`
	TakenAt    string `json:"takenAt,omitempty"`
}
type UploadResult struct {
	AlbumRef
	PhotoID      string `json:"photoId,omitempty"`
	UploadURL    string `json:"uploadUrl,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

func (s *Server) uploadPhoto(ctx context.Context, p principal, in PhotoUploadInput) (UploadResult, error) {
	id := in.AlbumID
	if !validKey(in.RequestKey) {
		return UploadResult{}, errAlbumInput
	}
	if in.DataBase64 == "" {
		err := s.withAlbum(ctx, p, uploadPhotoTool, id, func(albumTx) error { return nil })
		if err != nil {
			return UploadResult{}, err
		}
		return UploadResult{AlbumRef: in.AlbumRef, UploadURL: s.cfg.Origin + endpoint + "/albums/" + id + "/photos", Instructions: "POST multipart/form-data，使用同一 Bearer 凭据和 Idempotency-Key 请求头（值为 requestKey）。表单 file、alt 必填，caption、takenAt 可选。单图最大 50 MiB。"}, nil
	}
	if len(in.DataBase64) > base64.StdEncoding.EncodedLen(inlinePhotoLimit) {
		return UploadResult{}, albums.PhotoInputError("超过 1 MiB 请使用文件上传地址")
	}
	data, err := base64.StdEncoding.DecodeString(in.DataBase64)
	if err != nil || len(data) > inlinePhotoLimit {
		return UploadResult{}, errAlbumInput
	}
	return s.savePhoto(ctx, p, id, in.RequestKey, albums.PhotoUpload{Data: data, Filename: in.Filename, Alt: in.Alt, Caption: in.Caption, TakenAt: in.TakenAt})
}

func (s *Server) savePhoto(ctx context.Context, p principal, id, key string, in albums.PhotoUpload) (out UploadResult, err error) {
	if !validKey(key) {
		return out, errAlbumInput
	}
	out.AlbumID = id
	dataHash := sha256.Sum256(in.Data)
	meta, _ := json.Marshal([]string{in.Filename, in.Alt, in.Caption, in.TakenAt})
	hash := sha256.Sum256(append(meta, dataHash[:]...))
	lookup := func(a albumTx) error {
		prev, err := a.q.FindMCPUpload(ctx, dbgen.FindMCPUploadParams{AlbumID: a.id, ConnectionID: p.id, RequestKey: key})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(prev.PayloadHash, hash[:]) {
			return errKey
		}
		if !prev.PhotoID.Valid {
			return errAlbumMissing
		}
		out.PhotoID = prev.PhotoID.String()
		return nil
	}
	err = s.withAlbum(ctx, p, uploadPhotoTool, id, lookup)
	if err != nil || out.PhotoID != "" {
		return out, err
	}
	staged, err := s.albums.StagePhoto(ctx, in)
	if err != nil {
		return out, err
	}
	attached := false
	err = s.withAlbum(ctx, p, uploadPhotoTool, id, func(a albumTx) error {
		if err := lookup(a); err != nil || out.PhotoID != "" {
			return err
		}
		saved, err := staged.Attach(ctx, a.q, a.id, a.owner)
		if err != nil {
			return err
		}
		photo := saved.Photo.PhotoID
		if a.delegatedUpload {
			err = a.q.BindMCPDelegatedUpload(ctx, dbgen.BindMCPDelegatedUploadParams{AlbumID: a.id, ConnectionID: p.id, RequestKey: key, PayloadHash: hash[:], PhotoID: photo})
		} else {
			err = a.q.BindMCPUpload(ctx, dbgen.BindMCPUploadParams{AlbumID: a.id, RequestKey: key, PayloadHash: hash[:], PhotoID: photo})
		}
		if err != nil {
			return err
		}
		out.PhotoID = photo.String()
		attached = true
		return nil
	})
	if err != nil || !attached {
		staged.Cleanup(ctx)
	}
	return out, err
}

func (s *Server) multipartPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	info := mcpauth.TokenInfoFromContext(ctx)
	if info == nil {
		http.Error(w, "需要连接令牌", 401)
		return
	}
	conn, err := parseID(info.UserID)
	if err != nil {
		http.Error(w, "凭据无效", 401)
		return
	}
	p := principal{id: conn, albumScope: slices.Contains(info.Scopes, albumScope)}
	rawID := chi.URLParam(r, "id")
	q := dbgen.New(s.store.pool)
	if err := q.ObserveMCPClient(ctx, dbgen.ObserveMCPClientParams{ID: conn}); err != nil {
		http.Error(w, "无法记录客户端活动", 503)
		return
	}
	entry, err := q.StartMCPOperation(ctx, dbgen.StartMCPOperationParams{ConnectionID: conn, Operation: uploadPhotoTool})
	if err != nil {
		http.Error(w, "无法记录本次操作", 503)
		return
	}
	finish := dbgen.FinishMCPOperationParams{ID: entry, Outcome: "failed"}
	defer func() { finishOperation(ctx, q, finish) }()
	if err = s.withAlbum(ctx, p, uploadPhotoTool, rawID, func(albumTx) error { return nil }); err != nil {
		uploadError(w, err)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if !validKey(key) {
		uploadError(w, errAlbumInput)
		return
	}
	in, ok := albums.ReadPhotoUpload(w, r)
	if !ok {
		return
	}
	out, err := s.savePhoto(ctx, p, rawID, key, in)
	if err != nil {
		uploadError(w, err)
		return
	}
	finish.Outcome = "success"
	finish.AlbumID, _ = parseID(rawID)
	respond(w, http.StatusCreated, out)
}
func uploadError(w http.ResponseWriter, err error) {
	var bad albums.PhotoInputError
	switch {
	case errors.Is(err, errDenied):
		http.Error(w, errDenied.Error(), 403)
	case errors.Is(err, errPublishedUpload):
		http.Error(w, errPublishedUpload.Error(), http.StatusForbidden)
	case errors.Is(err, errAlbumMissing):
		http.Error(w, errAlbumMissing.Error(), 404)
	case errors.Is(err, errKey), errors.Is(err, errAlbumState):
		http.Error(w, err.Error(), 409)
	case errors.Is(err, errAlbumInput), errors.As(err, &bad):
		http.Error(w, err.Error(), 400)
	default:
		http.Error(w, "照片上传失败", 503)
	}
}
