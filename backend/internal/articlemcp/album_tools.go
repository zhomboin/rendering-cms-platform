package articlemcp

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/albums"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func (s *Server) registerAlbumTools(server *mcp.Server, permissions []string) {
	for _, tool := range permissions {
		switch tool {
		case createAlbumTool:
			registerTool(server, tool, "创建本连接的草稿相册，重试使用同一 requestKey。", s.createAlbum)
		case listAlbumsTool:
			registerTool(server, tool, "分页列出授权范围内的相册。", s.listAlbums)
		case getAlbumTool:
			registerTool(server, tool, "查看授权范围内的相册信息、封面和照片数。", s.getAlbum)
		case updateAlbumTool:
			registerTool(server, tool, "替换草稿相册的基本信息。首次发布后 slug 固定。", s.updateAlbum)
		case publishAlbumTool:
			registerTool(server, tool, "公开发布至少有一张照片的相册。", s.albumState(tool))
		case unpublishAlbumTool:
			registerTool(server, tool, "将本连接已发布相册下架为草稿。", s.albumState(tool))
		case listPhotosTool:
			registerTool(server, tool, "分页列出授权范围内的相册照片、顺序和图片地址。", s.listPhotos)
		case uploadPhotoTool:
			registerTool(server, tool, "上传照片到草稿相册。1 MiB 内可传 dataBase64 和 alt。省略数据可获取 50 MiB 文件上传地址及用法。", s.uploadPhoto)
		case updatePhotoTool:
			registerTool(server, tool, "替换照片的 alt、说明和拍摄时间。", s.updatePhoto)
		case orderPhotosTool:
			registerTool(server, tool, "按数组重新排序，须包含草稿相册内的每张照片。", s.orderPhotos)
		case coverTool:
			registerTool(server, tool, "设置草稿相册封面；省略 photoId 恢复默认封面。", s.cover)
		case deletePhotoTool:
			registerTool(server, tool, "删除草稿相册中的指定照片并清理图片文件。", s.deletePhoto)
		}
	}
}

type AlbumPage struct {
	Offset int32 `json:"offset,omitempty"`
	Limit  int32 `json:"limit,omitempty"`
}

func (p AlbumPage) limit() (int32, error) {
	if p.Offset < 0 || p.Limit < 0 || p.Limit > 100 {
		return 0, errAlbumInput
	}
	if p.Limit == 0 {
		return 24, nil
	}
	return p.Limit, nil
}

type AlbumSummary struct {
	AlbumRef
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Status string `json:"status"`
}
type AlbumList struct {
	Items []AlbumSummary `json:"items"`
}

func (s *Server) listAlbums(ctx context.Context, p principal, in AlbumPage) (out AlbumList, err error) {
	limit, err := in.limit()
	if err != nil {
		return out, err
	}
	err = s.withAlbum(ctx, p, listAlbumsTool, "", func(a albumTx) error {
		rows, err := a.q.ListMCPAlbums(ctx, dbgen.ListMCPAlbumsParams{ConnectionID: p.id, Offset: in.Offset, Limit: limit, ReadAll: a.readAll})
		if err != nil {
			return err
		}
		out.Items = make([]AlbumSummary, 0, len(rows))
		for _, r := range rows {
			out.Items = append(out.Items, AlbumSummary{AlbumRef: AlbumRef{r.AlbumID.String()}, Slug: r.Slug, Title: r.Title, Status: string(r.Status)})
		}
		return nil
	})
	return out, err
}

type PhotosPage struct {
	AlbumRef
	AlbumPage
}
type AlbumPhoto struct {
	PhotoID  string `json:"photoId"`
	Alt      string `json:"alt"`
	Caption  string `json:"caption,omitempty"`
	TakenAt  string `json:"takenAt,omitempty"`
	URL      string `json:"url"`
	Position int32  `json:"position"`
	Width    int32  `json:"width"`
	Height   int32  `json:"height"`
}
type PhotoList struct {
	Items []AlbumPhoto `json:"items"`
}

func (s *Server) listPhotos(ctx context.Context, p principal, in PhotosPage) (out PhotoList, err error) {
	limit, err := in.limit()
	if err != nil {
		return out, err
	}
	err = s.withAlbum(ctx, p, listPhotosTool, in.AlbumID, func(a albumTx) error {
		rows, err := a.q.ListMCPPhotos(ctx, dbgen.ListMCPPhotosParams{AlbumID: a.id, Offset: in.Offset, Limit: limit})
		if err != nil {
			return err
		}
		out.Items = make([]AlbumPhoto, 0, len(rows))
		for _, r := range rows {
			photo := AlbumPhoto{PhotoID: r.PhotoID.String(), Alt: r.Alt, Caption: r.Caption.String, URL: r.PublicUrl.String, Position: r.Position, Width: r.Width, Height: r.Height}
			if r.TakenAt.Valid {
				photo.TakenAt = r.TakenAt.Time.UTC().Format(time.RFC3339)
			}
			out.Items = append(out.Items, photo)
		}
		return nil
	})
	return out, err
}

type PhotoRef struct {
	AlbumRef
	PhotoID string `json:"photoId"`
}
type PhotoEdit struct {
	PhotoRef
	Alt     string `json:"alt"`
	Caption string `json:"caption,omitempty"`
	TakenAt string `json:"takenAt,omitempty"`
}
type PhotoOrder struct {
	AlbumRef
	PhotoIDs []string `json:"photoIds"`
}
type AlbumCover struct {
	AlbumRef
	PhotoID string `json:"photoId,omitempty"`
}

func (s *Server) updatePhoto(ctx context.Context, p principal, in PhotoEdit) (PhotoRef, error) {
	if albums.ValidatePhotoAlt(in.Alt) != nil || albums.ValidatePhotoCaption(in.Caption) != nil {
		return PhotoRef{}, errAlbumInput
	}
	var taken pgtype.Timestamptz
	if in.TakenAt != "" {
		t, err := time.Parse(time.RFC3339, in.TakenAt)
		if err != nil {
			return PhotoRef{}, errAlbumInput
		}
		taken = pgtype.Timestamptz{Time: t, Valid: true}
	}
	id, err := parseID(in.PhotoID)
	if err != nil {
		return PhotoRef{}, errAlbumMissing
	}
	err = s.withAlbum(ctx, p, updatePhotoTool, in.AlbumID, func(a albumTx) error {
		_, err := a.q.UpdateAlbumPhoto(ctx, dbgen.UpdateAlbumPhotoParams{AlbumID: a.id, PhotoID: id, Alt: in.Alt, Caption: optionalText(in.Caption), TakenAt: taken})
		return err
	})
	return in.PhotoRef, err
}
func (s *Server) orderPhotos(ctx context.Context, p principal, in PhotoOrder) (AlbumRef, error) {
	err := s.withAlbum(ctx, p, orderPhotosTool, in.AlbumID, func(a albumTx) error {
		_, err := a.handler().ReorderPhotos(ctx, a.id, in.PhotoIDs)
		return err
	})
	return in.AlbumRef, err
}
func (s *Server) cover(ctx context.Context, p principal, in AlbumCover) (AlbumRef, error) {
	var photo pgtype.UUID
	if in.PhotoID != "" {
		var err error
		photo, err = parseID(in.PhotoID)
		if err != nil {
			return AlbumRef{}, errAlbumMissing
		}
	}
	err := s.withAlbum(ctx, p, coverTool, in.AlbumID, func(a albumTx) error { return a.handler().SetCoverPhoto(ctx, a.id, photo) })
	return in.AlbumRef, err
}
func (s *Server) deletePhoto(ctx context.Context, p principal, in PhotoRef) (PhotoRef, error) {
	id, err := parseID(in.PhotoID)
	if err != nil {
		return PhotoRef{}, errAlbumMissing
	}
	var key string
	err = s.withAlbum(ctx, p, deletePhotoTool, in.AlbumID, func(a albumTx) error {
		var e error
		key, e = a.handler().RemovePhoto(ctx, a.id, id)
		return e
	})
	if err == nil {
		s.albums.CleanupObjectsAfterWrite(ctx, key)
	}
	return in, err
}
