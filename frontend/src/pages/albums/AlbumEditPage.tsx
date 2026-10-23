import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Divider,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import type { UploadRequestOption } from '@rc-component/upload/lib/interface';
import {
  ArrowLeftOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
  CloudUploadOutlined,
  DeleteOutlined,
  EditOutlined,
  PictureOutlined,
  SaveOutlined,
  StarFilled,
  StarOutlined,
  StopOutlined,
} from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createAdminAlbum,
  deleteAdminAlbumPhoto,
  getAdminAlbum,
  listAdminAlbumPhotos,
  photoContentTypes,
  publishAdminAlbum,
  reorderAdminAlbumPhotos,
  setAdminAlbumCover,
  unpublishAdminAlbum,
  updateAdminAlbum,
  updateAdminAlbumPhoto,
  uploadAdminAlbumPhoto,
} from '../../api/albums';
import type { AlbumPhoto } from '../../api/albums';

const { Title, Text } = Typography;
const { TextArea } = Input;

const maxPhotoBytes = 20 * 1024 * 1024;

interface AlbumFormValues {
  slug: string;
  title: string;
  titleEn?: string;
  description: string;
  descriptionEn?: string;
  tags?: string[];
}

interface PhotoEditValues {
  alt: string;
  caption?: string;
  takenAt?: Dayjs | null;
}

function defaultAltFor(filename: string) {
  const base = filename.replace(/\.[^.]+$/, '').replace(/[-_]+/g, ' ').trim();
  return base || filename;
}

export default function AlbumEditPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const isCreating = id === 'new';
  const [form] = Form.useForm<AlbumFormValues>();
  const [editForm] = Form.useForm<PhotoEditValues>();
  const [editingPhoto, setEditingPhoto] = useState<AlbumPhoto | null>(null);

  const albumQuery = useQuery({
    queryKey: ['admin-album', id],
    queryFn: () => getAdminAlbum(id),
    enabled: !isCreating,
  });
  const photosQuery = useQuery({
    queryKey: ['admin-album-photos', id],
    queryFn: () => listAdminAlbumPhotos(id),
    enabled: !isCreating,
  });
  const album = albumQuery.data;
  const photos = useMemo(() => photosQuery.data ?? [], [photosQuery.data]);

  useEffect(() => {
    if (album) {
      form.setFieldsValue({
        slug: album.slug,
        title: album.title,
        titleEn: album.titleEn ?? undefined,
        description: album.description,
        descriptionEn: album.descriptionEn ?? undefined,
        tags: album.tags,
      });
    }
  }, [album, form]);

  const invalidateAlbum = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin-album', id] });
    void queryClient.invalidateQueries({ queryKey: ['admin-album-photos', id] });
    void queryClient.invalidateQueries({ queryKey: ['admin-albums'] });
  };

  const saveMutation = useMutation({
    mutationFn: (values: AlbumFormValues) =>
      isCreating ? createAdminAlbum(values) : updateAdminAlbum(id, values),
    onSuccess: (saved) => {
      message.success(isCreating ? '相册已创建，可以开始上传照片' : '相册信息已保存');
      if (isCreating) {
        navigate(`/admin/albums/${saved.albumId}/edit`);
        return;
      }
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '相册保存失败');
    },
  });

  const publishMutation = useMutation({
    mutationFn: () => publishAdminAlbum(id),
    onSuccess: () => {
      message.success('相册已发布');
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '相册发布失败');
    },
  });

  const unpublishMutation = useMutation({
    mutationFn: () => unpublishAdminAlbum(id),
    onSuccess: () => {
      message.success('相册已下架');
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '相册下架失败');
    },
  });

  const uploadMutation = useMutation({
    mutationFn: (options: { file: File; alt: string }) =>
      uploadAdminAlbumPhoto(id, options.file, { alt: options.alt }),
    onSuccess: () => {
      message.success('照片已上传');
      invalidateAlbum();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '照片上传失败');
    },
  });

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

  const handleUpload = (options: UploadRequestOption) => {
    const { file } = options;
    const raw = file as File;
    if (!photoContentTypes.includes(raw.type)) {
      message.error(`${raw.name} 不是 PNG/JPEG/WebP 图片`);
      options.onError?.(new Error('unsupported type'));
      return;
    }
    if (raw.size > maxPhotoBytes) {
      message.error(`${raw.name} 超过 20MB 上限`);
      options.onError?.(new Error('file too large'));
      return;
    }
    uploadMutation.mutate(
      { file: raw, alt: defaultAltFor(raw.name) },
      {
        onSuccess: () => options.onSuccess?.({}),
        onError: (uploadError) => options.onError?.(uploadError),
      },
    );
  };

  const openPhotoEdit = (photo: AlbumPhoto) => {
    setEditingPhoto(photo);
    editForm.setFieldsValue({
      alt: photo.alt,
      caption: photo.caption ?? undefined,
      takenAt: photo.takenAt ? dayjs(photo.takenAt) : undefined,
    });
  };

  const isPublished = album?.status === 'published';

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 20 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/admin/albums')}>
          返回列表
        </Button>
        {!isCreating && isPublished && (
          <Popconfirm
            title="确认下架该相册？"
            description="下架后公开接口立即返回 404。"
            okText="下架"
            okButtonProps={{ danger: true }}
            cancelText="取消"
            onConfirm={() => unpublishMutation.mutate()}
          >
            <Button danger icon={<StopOutlined />}>下架</Button>
          </Popconfirm>
        )}
        {!isCreating && !isPublished && (
          <Popconfirm
            title="确认发布该相册？"
            description="发布后立即出现在公开接口；已发布相册的 slug 不可再修改。"
            okText="发布"
            cancelText="取消"
            onConfirm={() => publishMutation.mutate()}
          >
            <Button type="primary" icon={<CloudUploadOutlined />} style={{ backgroundColor: '#16A34A' }}>
              发布相册
            </Button>
          </Popconfirm>
        )}
      </Space>

      {albumQuery.error && (
        <Alert
          type="error"
          showIcon
          message={albumQuery.error instanceof Error ? albumQuery.error.message : '相册读取失败'}
          style={{ marginBottom: 20 }}
        />
      )}

      <Card loading={!isCreating && albumQuery.isLoading} style={{ marginBottom: 24 }}>
        <Title level={2} style={{ marginTop: 0, marginBottom: 20, fontSize: 18 }}>
          {isCreating ? '新建相册' : `相册信息${album ? ` · ${album.title}` : ''}`}
        </Title>
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => saveMutation.mutate(values)}
          requiredMark
        >
          <Row gutter={16}>
            <Col xs={24} md={8}>
              <Form.Item
                name="slug"
                label="slug（公开路由 /photos/{slug}）"
                rules={[
                  { required: true, message: '请输入 slug' },
                  {
                    pattern: /^[a-z0-9]+(-[a-z0-9]+)*$/,
                    message: '仅小写字母、数字和中划线，发布后不可修改',
                  },
                ]}
              >
                <Input placeholder="city-nights" disabled={isPublished} />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item name="title" label="标题（中文，必填）" rules={[{ required: true, message: '请输入标题' }]}>
                <Input placeholder="城市夜色" />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item name="titleEn" label="英文标题（可选，镜像页回退中文）">
                <Input placeholder="City Nights" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item name="description" label="导语（中文，必填）" rules={[{ required: true, message: '请输入导语' }]}>
                <TextArea rows={3} placeholder="一组夜景。" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item name="descriptionEn" label="英文导语（可选）">
                <TextArea rows={3} placeholder="A set of night shots." />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="tags" label="标签（仅展示用途）">
            <Select mode="tags" placeholder="photography、city…" tokenSeparators={[',', ' ']} open={false} />
          </Form.Item>
          <Button
            type="primary"
            htmlType="submit"
            icon={<SaveOutlined />}
            loading={saveMutation.isPending}
            style={{ backgroundColor: '#4F46E5' }}
          >
            {isCreating ? '创建相册并继续' : '保存信息'}
          </Button>
        </Form>
      </Card>

      {!isCreating && (
        <Card loading={photosQuery.isLoading}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, gap: 12, flexWrap: 'wrap' }}>
            <Title level={2} style={{ margin: 0, fontSize: 18 }}>
              照片（{photos.length} 张{album?.coverPhotoId ? '，已指定封面' : '，默认封面为第一张'}）
            </Title>
            <Text type="secondary">
              上传后服务端自动提取尺寸、剥离 EXIF GPS；alt 默认取文件名，请按需编辑。
            </Text>
          </div>

          <Upload.Dragger
            multiple
            showUploadList={false}
            accept={photoContentTypes.join(',')}
            customRequest={handleUpload}
            disabled={uploadMutation.isPending}
          >
            <p className="ant-upload-drag-icon"><PictureOutlined /></p>
            <p className="ant-upload-text">点击或拖拽照片到这里上传</p>
            <p className="ant-upload-hint">支持 PNG / JPEG / WebP，单张不超过 20MB</p>
          </Upload.Dragger>

          <Divider />

          {photos.length === 0 ? (
            <Alert type="info" showIcon message="还没有照片；发布相册前至少需要上传一张。" />
          ) : (
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
                          <img
                            src={photo.url}
                            alt={photo.alt}
                            style={{ height: 160, objectFit: 'cover' }}
                            loading="lazy"
                          />
                        ) : undefined
                      }
                      actions={[
                        <Button key="up" type="text" size="small" icon={<ArrowUpOutlined />} disabled={index === 0 || reorderMutation.isPending} onClick={() => movePhoto(index, -1)} />,
                        <Button key="down" type="text" size="small" icon={<ArrowDownOutlined />} disabled={index === photos.length - 1 || reorderMutation.isPending} onClick={() => movePhoto(index, 1)} />,
                        <Button key="edit" type="text" size="small" icon={<EditOutlined />} onClick={() => openPhotoEdit(photo)} />,
                        <Button
                          key="cover"
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
                          <Button type="text" size="small" danger icon={<DeleteOutlined />} />
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
                    </Card>
                  </Col>
                );
              })}
            </Row>
          )}
        </Card>
      )}

      <Modal
        title="编辑照片信息"
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
            label="alt（无障碍必填）"
            rules={[{ required: true, message: '请输入 alt 描述' }]}
          >
            <Input placeholder="夜色中的天桥" />
          </Form.Item>
          <Form.Item name="caption" label="说明文字（灯箱中展示，可选）">
            <Input placeholder="拍于江边" />
          </Form.Item>
          <Form.Item name="takenAt" label="拍摄时间（可选）">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
