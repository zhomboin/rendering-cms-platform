import { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Alert, Button, Card, Col, Form, Input, Popconfirm, Row, Select, Space, Typography, message } from 'antd';
import { ArrowLeftOutlined, CloudUploadOutlined, PictureOutlined, SaveOutlined, StopOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createAdminAlbum, getAdminAlbum, publishAdminAlbum, unpublishAdminAlbum, updateAdminAlbum } from '../../api/albums';

const { Title } = Typography;
const { TextArea } = Input;

interface AlbumFormValues {
  slug: string;
  title: string;
  titleEn?: string;
  description: string;
  descriptionEn?: string;
  tags?: string[];
}

export default function AlbumEditPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const isCreating = !id;
  const [form] = Form.useForm<AlbumFormValues>();

  const albumQuery = useQuery({
    queryKey: ['admin-album', id],
    queryFn: () => getAdminAlbum(id),
    enabled: !isCreating,
  });
  const album = albumQuery.data;

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
      message.success(isCreating ? '相册已创建' : '相册信息已保存');
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

  const isPublished = album?.status === 'published';

  return (
    <div style={{ padding: 24 }}>
      <Space wrap style={{ marginBottom: 20 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/admin/albums')}>
          返回列表
        </Button>
        {!isCreating && album && (
          <Button aria-label="管理图片" icon={<PictureOutlined />} onClick={() => navigate(`/admin/albums/${id}/photos`)}>管理图片</Button>
        )}
        {!isCreating && isPublished && (
          <Popconfirm
            title="确认下架该相册？"
            description="下架后不再公开展示。"
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
            description="发布后公开展示，访问标识不可修改。"
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
          {isCreating ? '新建相册' : '相册信息'}
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
                label="访问标识"
                rules={[
                  { required: true, message: '请输入访问标识' },
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
              <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
                <Input placeholder="城市夜色" />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item name="titleEn" label="英文标题">
                <Input placeholder="City Nights" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item name="description" label="导语" rules={[{ required: true, message: '请输入导语' }]}>
                <TextArea rows={3} placeholder="一组夜景。" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item name="descriptionEn" label="英文导语">
                <TextArea rows={3} placeholder="A set of night shots." />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="tags" label="标签">
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

    </div>
  );
}
