package articlemcp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

var errUploadTargets = errors.New("请选择已发布相册，并勾选上传照片")

type uploadAlbumIDs []pgtype.UUID

func (ids *uploadAlbumIDs) UnmarshalJSON(data []byte) error {
	var values []pgtype.UUID
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return errUploadTargets
	}
	seen := make(map[pgtype.UUID]bool, len(values))
	for _, id := range values {
		if !id.Valid || seen[id] {
			return errUploadTargets
		}
		seen[id] = true
	}
	*ids = values
	return nil
}

// 调用方在同一事务中创建或更新连接，持有连接行锁直到授权保存完毕。
func replaceUploadGrants(ctx context.Context, q *dbgen.Queries, id pgtype.UUID, targets uploadAlbumIDs, permissions []string) error {
	if !slices.Contains(permissions, uploadPhotoTool) {
		if len(targets) > 0 {
			return errUploadTargets
		}
		targets = uploadAlbumIDs{}
	}
	if targets == nil {
		return nil
	}
	if err := q.RemoveMCPUploadGrantsExcept(ctx, dbgen.RemoveMCPUploadGrantsExceptParams{ConnectionID: id, AlbumIds: targets}); err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	if err := q.AddMCPUploadGrants(ctx, dbgen.AddMCPUploadGrantsParams{ConnectionID: id, AlbumIds: targets}); err != nil {
		return err
	}
	count, err := q.CountMCPUploadGrants(ctx, id)
	if err != nil {
		return err
	}
	if count != int64(len(targets)) {
		return errUploadTargets
	}
	return nil
}
