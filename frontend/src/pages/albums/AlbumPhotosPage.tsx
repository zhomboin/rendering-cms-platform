import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Alert, Button, Card, Col, DatePicker, Divider, Form, Input, Modal, Popconfirm, Row, Space, Tag, Typography, Image, message } from 'antd';
import { ArrowLeftOutlined, ArrowDownOutlined, ArrowUpOutlined, CopyOutlined, DeleteOutlined, EditOutlined, StarFilled, StarOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteAdminAlbumPhoto, getAdminAlbum, listAdminAlbumPhotos, reorderAdminAlbumPhotos, setAdminAlbumCover, updateAdminAlbumPhoto } from '../../api/albums';
import type { AlbumPhoto } from '../../api/albums';
import AlbumPhotoUpload from './AlbumPhotoUpload';

const { Title, Text } = Typography;

interface PhotoEditValues {
  alt: string;
  caption?: string;
  takenAt?: Dayjs | null;
}

export default function AlbumPhotosPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [editForm] = Form.useForm<PhotoEditValues>();
  const [editingPhoto, setEditingPhoto] = useState<AlbumPhoto | null>(null);
  const albumQuery = useQuery({ queryKey: ['admin-album', id], queryFn: () => getAdminAlbum(id), enabled: !!id });
  const photosQuery = useQuery({ queryKey: ['admin-album-photos', id], queryFn: () => listAdminAlbumPhotos(id), enabled: !!id && !!albumQuery.data });
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
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin-album-photos', id] });
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '照片排序失败');
      void queryClient.invalidateQueries({ queryKey: ['admin-album-photos', id] });
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

  const movePhoto = (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= photos.length) return;
    const next = [...photos];
    const [moved] = next.splice(index, 1);
    next.splice(target, 0, moved);
    reorderMutation.mutate(next.map((photo) => photo.photoId));
  };

  const openPhotoEdit = (photo: AlbumPhoto) => {
    setEditingPhoto(photo);
    editForm.setFieldsValue({
      alt: photo.alt,
      caption: photo.caption ?? undefined,
      takenAt: photo.takenAt ? dayjs(photo.takenAt) : undefined,
    });
  };

  const copyPhotoURL = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      message.success('图片 URL 已复制');
    } catch {
      message.error('复制失败，请选中图片 URL 手动复制');
    }
  };
  const error = albumQuery.error ?? photosQuery.error;
  return (
    <div style={{ padding: 24 }}>
      <Space wrap style={{ marginBottom: 20 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/admin/albums')}>返回列表</Button>
        <Button aria-label="编辑基本信息" icon={<EditOutlined />} onClick={() => navigate(`/admin/albums/${id}/edit`)}>编辑基本信息</Button>
      </Space>
      <Title level={1} style={{ fontSize: 24 }}>图片管理</Title>
      {error ? <Alert type="error" showIcon message={error instanceof Error ? error.message : '图片读取失败'} /> : (
        <Card loading={albumQuery.isLoading || photosQuery.isLoading}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, gap: 12, flexWrap: 'wrap' }}>
            <Space size={8}>
              <Title level={2} style={{ margin: 0, fontSize: 18 }}>照片</Title>
              <Text type="secondary">{photos.length} 张</Text>
            </Space>
          </div>
          <AlbumPhotoUpload key={id} albumId={id} disabled={!album} onUploaded={invalidateAlbum} />

          <Divider />

          {photos.length === 0 ? (
            <Alert type="info" showIcon message="暂无照片" />
          ) : (
            <Image.PreviewGroup>
              <Row gutter={[16, 16]}>
                {photos.map((photo, index) => {
                  const isCover = album?.coverPhotoId
                    ? album.coverPhotoId === photo.photoId
                    : index === 0;
                  return (
                    <Col key={photo.photoId} xs={24} sm={12} md={8} lg={6}>
                      <Card
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
                        actions={[
                          <Button aria-label="上移照片" key="up" type="text" size="small" icon={<ArrowUpOutlined />} disabled={index === 0 || reorderMutation.isPending} onClick={() => movePhoto(index, -1)} />,
                          <Button aria-label="下移照片" key="down" type="text" size="small" icon={<ArrowDownOutlined />} disabled={index === photos.length - 1 || reorderMutation.isPending} onClick={() => movePhoto(index, 1)} />,
                          <Button aria-label="编辑照片" key="edit" type="text" size="small" icon={<EditOutlined />} onClick={() => openPhotoEdit(photo)} />,
                          <Button
                            key="cover"
                            aria-label="设置照片封面"
                            type="text"
                            size="small"
                            icon={isCover ? <StarFilled style={{ color: '#F59E0B' }} /> : <StarOutlined />}
                            onClick={() => coverMutation.mutate(isCover ? null : photo.photoId)}
                          />,
                          <Popconfirm
                            key="delete"
                            title="确认删除该照片？"
                            okText="删除"
                            okButtonProps={{ danger: true }}
                            cancelText="取消"
                            onConfirm={() => deletePhotoMutation.mutate(photo.photoId)}
                          >
                            <Button aria-label="删除照片" type="text" size="small" danger icon={<DeleteOutlined />} />
                          </Popconfirm>,
                        ]}
                      >
                        <Card.Meta
                          title={
                            <Space size={6}>
                              <Text ellipsis style={{ maxWidth: 160 }}>{photo.alt}</Text>
                              {isCover && <Tag color="gold">封面</Tag>}
                            </Space>
                          }
                          description={
                            <Text type="secondary" style={{ fontSize: 12 }}>
                              {photo.width}×{photo.height}
                              {photo.takenAt ? ` · ${dayjs(photo.takenAt).format('YYYY-MM-DD')}` : ''}
                            </Text>
                          }
                        />
                        {photo.url ? (
                          <Space.Compact style={{ width: '100%', marginTop: 12 }}>
                            <Input aria-label={`${photo.alt}的图片 URL`} readOnly value={photo.url} onFocus={(event) => event.target.select()} />
                            <Button aria-label={`复制${photo.alt}的图片 URL`} icon={<CopyOutlined />} onClick={() => void copyPhotoURL(photo.url!)}>复制</Button>
                          </Space.Compact>
                        ) : <Text type="secondary">暂无图片 URL，请检查公开访问地址配置</Text>}
                      </Card>
                    </Col>
                  );
                })}
              </Row>
            </Image.PreviewGroup>
          )}
        </Card>
      )}
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
