import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  App,
  Alert,
  Button,
  Card,
  DatePicker,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Space,
  Typography,
  Image,
} from 'antd';
import { ArrowLeftOutlined, EditOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  deleteAdminAlbumPhoto,
  getAdminAlbum,
  listAdminAlbumPhotos,
  reorderAdminAlbumPhotos,
  setAdminAlbumCover,
  updateAdminAlbumPhoto,
} from '../../api/albums';
import type { AlbumPhoto } from '../../api/albums';
import AlbumPhotoUpload from './AlbumPhotoUpload';
import { AlbumPhotoCard } from './AlbumPhotoCard';
import './AlbumPhotosPage.css';

const { Title, Text } = Typography;

interface PhotoEditValues {
  alt: string;
  caption?: string;
  takenAt?: Dayjs | null;
}

export default function AlbumPhotosPage() {
  const { id = '' } = useParams();
  return <AlbumPhotos key={id} id={id} />;
}

function AlbumPhotos({ id }: { id: string }) {
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [editForm] = Form.useForm<PhotoEditValues>();
  const [editingPhoto, setEditingPhoto] = useState<AlbumPhoto | null>(null);
  const [linkPhoto, setLinkPhoto] = useState<AlbumPhoto | null>(null);
  const [movingPhoto, setMovingPhoto] = useState<AlbumPhoto | null>(null);
  const [position, setPosition] = useState<number | null>(null);
  const albumQuery = useQuery({
    queryKey: ['admin-album', id],
    queryFn: () => getAdminAlbum(id),
    enabled: !!id,
  });
  const photosQuery = useQuery({
    queryKey: ['admin-album-photos', id],
    queryFn: () => listAdminAlbumPhotos(id),
    enabled: !!id && !!albumQuery.data,
  });
  const album = albumQuery.data;
  const photos = useMemo(() => photosQuery.data ?? [], [photosQuery.data]);
  const invalidateAlbum = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin-album', id] });
    void queryClient.invalidateQueries({ queryKey: ['admin-album-photos', id] });
    void queryClient.invalidateQueries({ queryKey: ['admin-albums'] });
  };
  const deletePhotoMutation = useMutation({
    mutationFn: (photoId: string) => deleteAdminAlbumPhoto(id, photoId),
    onSuccess: () => {
      message.success('照片已删除');
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '照片删除失败');
    },
  });

  const reorderMutation = useMutation({
    mutationFn: (photoIds: string[]) => reorderAdminAlbumPhotos(id, photoIds),
    onSuccess: async (orderedPhotos) => {
      await queryClient.cancelQueries({ queryKey: ['admin-album-photos', id] });
      queryClient.setQueryData(['admin-album-photos', id], orderedPhotos);
      setMovingPhoto(null);
      message.success('照片顺序已更新');
    },
    onError: async (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '照片排序失败');
      await queryClient.invalidateQueries({ queryKey: ['admin-album-photos', id] });
    },
  });

  const coverMutation = useMutation({
    mutationFn: (photoId: string | null) => setAdminAlbumCover(id, photoId),
    onSuccess: () => {
      message.success('封面已更新');
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '封面设置失败');
    },
  });

  const photoEditMutation = useMutation({
    mutationFn: (input: { photoId: string; values: PhotoEditValues }) =>
      updateAdminAlbumPhoto(id, input.photoId, {
        alt: input.values.alt,
        caption: input.values.caption,
        takenAt: input.values.takenAt ? input.values.takenAt.toISOString() : null,
      }),
    onSuccess: () => {
      message.success('照片信息已更新');
      setEditingPhoto(null);
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '照片更新失败');
    },
  });

  const movingIndex = photos.findIndex((photo) => photo.photoId === movingPhoto?.photoId);
  const canMove =
    movingIndex >= 0 &&
    position !== null &&
    Number.isInteger(position) &&
    position >= 1 &&
    position <= photos.length &&
    position !== movingIndex + 1;

  const movePhoto = () => {
    if (!canMove || position === null || reorderMutation.isPending || photosQuery.isFetching)
      return;
    const photoIds = photos.map(({ photoId }) => photoId);
    const [moved] = photoIds.splice(movingIndex, 1);
    photoIds.splice(position - 1, 0, moved);
    reorderMutation.mutate(photoIds);
  };

  const openPhotoEdit = (photo: AlbumPhoto) => {
    setEditingPhoto(photo);
    editForm.setFieldsValue({
      alt: photo.alt,
      caption: photo.caption ?? undefined,
      takenAt: photo.takenAt ? dayjs(photo.takenAt) : undefined,
    });
  };

  const copyPhotoURL = async (photo: AlbumPhoto) => {
    if (!photo.url) return;
    try {
      await navigator.clipboard.writeText(photo.url);
      message.success('图片 URL 已复制');
    } catch {
      setLinkPhoto(photo);
      message.error('复制失败，请选中图片 URL 手动复制');
    }
  };
  const error = albumQuery.error ?? photosQuery.error;
  return (
    <div>
      <Space wrap style={{ marginBottom: 20 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/admin/albums')}>
          返回列表
        </Button>
        <Button
          aria-label="编辑基本信息"
          icon={<EditOutlined />}
          onClick={() => navigate(`/admin/albums/${id}/edit`)}
        >
          编辑基本信息
        </Button>
      </Space>
      <Title level={1} style={{ fontSize: 24 }}>
        图片管理
      </Title>
      {error ? (
        <Alert
          type="error"
          showIcon
          message={error instanceof Error ? error.message : '图片读取失败'}
        />
      ) : (
        <Card loading={albumQuery.isLoading || photosQuery.isLoading}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: 16,
              gap: 12,
              flexWrap: 'wrap',
            }}
          >
            <Space size={8}>
              <Title level={2} style={{ margin: 0, fontSize: 18 }}>
                照片
              </Title>
              <Text type="secondary">{photos.length} 张</Text>
            </Space>
          </div>
          <AlbumPhotoUpload key={id} albumId={id} disabled={!album} onUploaded={invalidateAlbum} />

          <Divider />

          {photos.length === 0 ? (
            <Alert type="info" showIcon message="暂无照片" />
          ) : (
            <Image.PreviewGroup>
              <div className="cms-photo-grid">
                {photos.map((photo, index) => (
                  <AlbumPhotoCard
                    key={photo.photoId}
                    photo={photo}
                    isCover={
                      album?.coverPhotoId ? album.coverPhotoId === photo.photoId : index === 0
                    }
                    hasExplicitCover={album?.coverPhotoId === photo.photoId}
                    canReorder={photos.length > 1 && !reorderMutation.isPending}
                    canSetCover={!coverMutation.isPending}
                    onEdit={() => openPhotoEdit(photo)}
                    onCopy={() => void copyPhotoURL(photo)}
                    onViewLink={() => setLinkPhoto(photo)}
                    onMove={() => {
                      setPosition(index + 1);
                      setMovingPhoto(photo);
                    }}
                    onCover={() =>
                      coverMutation.mutate(
                        album?.coverPhotoId === photo.photoId ? null : photo.photoId,
                      )
                    }
                    onDelete={() =>
                      modal.confirm({
                        title: '删除照片？',
                        content: `「${photo.alt}」将从相册移除。`,
                        okText: '删除照片',
                        okButtonProps: { danger: true },
                        cancelText: '取消',
                        onOk: () => deletePhotoMutation.mutateAsync(photo.photoId),
                      })
                    }
                  />
                ))}
              </div>
            </Image.PreviewGroup>
          )}
        </Card>
      )}
      <Modal
        title="调整照片顺序"
        open={movingPhoto !== null}
        destroyOnHidden
        onCancel={() => setMovingPhoto(null)}
        onOk={movePhoto}
        okText="保存顺序"
        cancelText="取消"
        confirmLoading={reorderMutation.isPending}
        okButtonProps={{ disabled: !canMove || photosQuery.isFetching }}
        cancelButtonProps={{ disabled: reorderMutation.isPending }}
        closable={!reorderMutation.isPending}
        keyboard={!reorderMutation.isPending}
        mask={{ closable: !reorderMutation.isPending }}
      >
        <p className="cms-photo-dialog-text">{movingPhoto?.alt}</p>
        <label htmlFor="album-photo-position">目标位置（1–{photos.length}）</label>
        <InputNumber
          id="album-photo-position"
          className="cms-photo-position"
          min={1}
          max={photos.length}
          value={position}
          onChange={setPosition}
          onPressEnter={movePhoto}
          disabled={reorderMutation.isPending}
        />
      </Modal>
      <Modal
        title="图片链接"
        open={linkPhoto !== null}
        destroyOnHidden
        onCancel={() => setLinkPhoto(null)}
        footer={null}
      >
        <p className="cms-photo-dialog-text">{linkPhoto?.url}</p>
      </Modal>
      <Modal
        title="编辑照片信息"
        okText="保存照片"
        cancelText="取消"
        open={editingPhoto !== null}
        onCancel={() => setEditingPhoto(null)}
        onOk={() => {
          if (!editingPhoto) return;
          editForm
            .validateFields()
            .then((values) => photoEditMutation.mutate({ photoId: editingPhoto.photoId, values }))
            .catch(() => undefined);
        }}
        confirmLoading={photoEditMutation.isPending}
        destroyOnHidden
      >
        <Form form={editForm} layout="vertical">
          <Form.Item
            name="alt"
            label="图片描述"
            rules={[{ required: true, message: '请输入图片描述' }]}
          >
            <Input placeholder="夜色中的天桥" />
          </Form.Item>
          <Form.Item name="caption" label="说明">
            <Input placeholder="拍于江边" />
          </Form.Item>
          <Form.Item name="takenAt" label="拍摄时间">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
