import { Button, Card, Dropdown, Image, Tag, Typography } from 'antd';
import type { MenuProps } from 'antd';
import {
  CopyOutlined,
  DeleteOutlined,
  EllipsisOutlined,
  LinkOutlined,
  SortAscendingOutlined,
  StarOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import type { AlbumPhoto } from '../../api/albums';

interface PhotoCardProps {
  photo: AlbumPhoto;
  isCover: boolean;
  hasExplicitCover: boolean;
  canReorder: boolean;
  canSetCover: boolean;
  onEdit: () => void;
  onCopy: () => void;
  onViewLink: () => void;
  onMove: () => void;
  onCover: () => void;
  onDelete: () => void;
}

export function AlbumPhotoCard({
  photo,
  isCover,
  hasExplicitCover,
  canReorder,
  canSetCover,
  onEdit,
  onCopy,
  onViewLink,
  onMove,
  onCover,
  onDelete,
}: PhotoCardProps) {
  const menuItems: MenuProps['items'] = [
    {
      key: 'move',
      icon: <SortAscendingOutlined aria-hidden="true" />,
      label: '调整顺序',
      disabled: !canReorder,
      onClick: onMove,
    },
    {
      key: 'cover',
      icon: <StarOutlined aria-hidden="true" />,
      label: hasExplicitCover ? '恢复自动封面' : '设为封面',
      disabled: !canSetCover,
      onClick: onCover,
    },
    {
      key: 'url',
      icon: <LinkOutlined aria-hidden="true" />,
      label: '查看图片链接',
      disabled: !photo.url,
      onClick: onViewLink,
    },
    { type: 'divider' },
    {
      key: 'delete',
      icon: <DeleteOutlined aria-hidden="true" />,
      label: '删除照片',
      danger: true,
      onClick: onDelete,
    },
  ];

  return (
    <Card
      className="cms-photo-card"
      size="small"
      cover={
        photo.url ? (
          <Image
            src={photo.url}
            aria-label={'预览' + photo.alt}
            width="100%"
            height={160}
            preview={{ cover: '预览' }}
            alt={photo.alt}
            style={{ height: 160, objectFit: 'cover' }}
            loading="lazy"
          />
        ) : undefined
      }
    >
      <Card.Meta
        title={
          <div className="cms-photo-title">
            <Typography.Text ellipsis={{ tooltip: photo.alt }}>{photo.alt}</Typography.Text>
            {isCover && <Tag color="warning">封面</Tag>}
          </div>
        }
        description={
          <>
            {photo.width}×{photo.height}
            {photo.takenAt ? ` · ${dayjs(photo.takenAt).format('YYYY-MM-DD')}` : ''}
          </>
        }
      />
      {!photo.url && (
        <Typography.Text type="secondary">暂无图片 URL，请检查公开访问地址配置</Typography.Text>
      )}
      <div className="cms-photo-actions">
        <Button type="text" aria-label={`编辑${photo.alt}`} onClick={onEdit}>
          编辑
        </Button>
        {photo.url && (
          <Button
            type="text"
            icon={<CopyOutlined />}
            aria-label={`复制${photo.alt}的图片 URL`}
            onClick={onCopy}
          >
            复制链接
          </Button>
        )}
        <Dropdown
          autoFocus
          destroyOnHidden
          menu={{ items: menuItems, 'aria-label': `${photo.alt}的照片操作` }}
          trigger={['click']}
        >
          <Button
            type="text"
            icon={<EllipsisOutlined />}
            aria-label={`更多操作：${photo.alt}`}
            aria-haspopup="menu"
          />
        </Dropdown>
      </div>
    </Card>
  );
}
