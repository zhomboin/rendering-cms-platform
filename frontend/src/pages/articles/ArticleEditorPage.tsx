import { useEffect, useMemo, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Form, Input, Select, Button, Typography, Modal, Space, message, Alert, Skeleton } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createAdminArticle,
  listAdminArticles,
  publishAdminArticle,
  updateAdminArticle,
} from '../../api/articles';
import type { AdminArticlePayload, AdminArticleRecord, ArticleFormData } from '../../api/articles';

const { Title } = Typography;
const { TextArea } = Input;

const initialFormData: ArticleFormData = {
  title: '',
  slug: '',
  summary: '',
  tags: [],
  bodyMdx: '',
  coverImageUrl: '',
};

function toPayload(values: ArticleFormData): AdminArticlePayload {
  return {
    slug: values.slug,
    title: values.title,
    summary: values.summary ?? '',
    bodyMdx: values.bodyMdx,
    tags: values.tags ?? [],
    featured: false,
    coverImageUrl: values.coverImageUrl ?? '',
  };
}

export default function ArticleEditorPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ArticleFormData>();
  const [publishModalOpen, setPublishModalOpen] = useState(false);
  const isEdit = Boolean(id);

  const articlesQuery = useQuery({
    queryKey: ['admin-articles'],
    queryFn: listAdminArticles,
    enabled: isEdit,
  });

  const currentArticle = useMemo(
    () => articlesQuery.data?.find((article) => article.articleId === id),
    [articlesQuery.data, id],
  );

  useEffect(() => {
    if (!isEdit) {
      form.setFieldsValue(initialFormData);
      return;
    }
    if (currentArticle) {
      form.setFieldsValue({
        title: currentArticle.title,
        slug: currentArticle.slug,
        summary: currentArticle.summary,
        tags: currentArticle.tags,
        bodyMdx: currentArticle.bodyMdx,
        coverImageUrl: currentArticle.coverImageUrl ?? '',
      });
    }
  }, [currentArticle, form, isEdit]);

  const saveMutation = useMutation({
    mutationFn: async (values: ArticleFormData) => {
      if (isEdit && id) {
        return updateAdminArticle(id, toPayload(values));
      }
      return createAdminArticle(toPayload(values));
    },
    onSuccess: async (article) => {
      message.success('草稿已保存');
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      if (!isEdit) navigate(`/admin/articles/${article.articleId}/edit`, { replace: true });
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '草稿保存失败');
    },
  });

  const publishMutation = useMutation({
    mutationFn: async (values: ArticleFormData) => {
      const saved = isEdit && id
        ? await updateAdminArticle(id, toPayload(values))
        : await createAdminArticle(toPayload(values));
      return publishAdminArticle(saved.articleId);
    },
    onSuccess: async () => {
      message.success('文章已发布');
      setPublishModalOpen(false);
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      navigate('/admin/articles');
    },
    onError: (error) => {
      setPublishModalOpen(false);
      message.error(error instanceof Error ? error.message : '文章发布失败');
    },
  });

  const handleSaveDraft = async () => {
    const values = await form.validateFields();
    saveMutation.mutate(values);
  };

  const handlePublish = async () => {
    const values = await form.validateFields();
    publishMutation.mutate(values);
  };

  if (isEdit && articlesQuery.isLoading) {
    return (
      <div style={{ padding: 24 }}>
        <Skeleton active paragraph={{ rows: 12 }} />
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      <Title level={1} style={{ margin: '0 0 24px', fontSize: 24, fontWeight: 700 }}>
        {isEdit ? '编辑文章' : '新建文章'}
      </Title>

      {articlesQuery.error && (
        <Alert type="error" showIcon message={articlesQuery.error instanceof Error ? articlesQuery.error.message : '文章读取失败'} style={{ marginBottom: 20 }} />
      )}

      <Card style={{ borderRadius: 8, border: '1px solid #E2E8F0', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }} styles={{ body: { padding: 32 } }}>
        <Form form={form} layout="vertical" initialValues={initialFormData} style={{ maxWidth: 800 }}>
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入文章标题' }]}>
            <Input placeholder="输入文章标题" size="large" />
          </Form.Item>

          <Form.Item
            name="slug"
            label="Slug"
            rules={[
              { required: true, message: '请输入 URL Slug' },
              { pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/, message: 'Slug 只能使用小写字母、数字和中划线' },
            ]}
          >
            <Input addonBefore="/articles/" placeholder="my-article-slug" size="large" />
          </Form.Item>

          <Form.Item name="summary" label="摘要">
            <TextArea rows={3} placeholder="文章摘要" showCount maxLength={300} />
          </Form.Item>

          <Form.Item name="tags" label="标签">
            <Select mode="tags" placeholder="输入标签后按回车添加" style={{ width: '100%' }} tokenSeparators={[',', '，']} />
          </Form.Item>

          <Form.Item name="bodyMdx" label="MDX 正文" rules={[{ required: true, message: '请输入文章正文' }]}>
            <TextArea rows={14} placeholder="使用 Markdown/MDX 格式编写文章正文" style={{ fontFamily: "'Fira Code', 'JetBrains Mono', 'SF Mono', Consolas, monospace" }} />
          </Form.Item>

          <Form.Item name="coverImageUrl" label="封面图片 URL">
            <Input placeholder="https://example.com/image.jpg" />
          </Form.Item>
        </Form>
      </Card>

      <div
        style={{
          position: 'sticky',
          bottom: 0,
          marginTop: 24,
          padding: '16px 0',
          background: '#FFFFFF',
          borderTop: '1px solid #E2E8F0',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <Button onClick={() => navigate('/admin/articles')}>取消</Button>
        <Space size={12}>
          <Button onClick={handleSaveDraft} loading={saveMutation.isPending} style={{ borderColor: '#4F46E5', color: '#4F46E5' }}>
            保存草稿
          </Button>
          <Button
            type="primary"
            size="large"
            loading={publishMutation.isPending}
            style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5', fontWeight: 600 }}
            onClick={() => setPublishModalOpen(true)}
          >
            发布
          </Button>
        </Space>
      </div>

      <Modal
        title="确认发布"
        open={publishModalOpen}
        onOk={handlePublish}
        onCancel={() => setPublishModalOpen(false)}
        okText="确认发布"
        cancelText="取消"
        confirmLoading={publishMutation.isPending}
      >
        <p>确定要发布这篇文章吗？发布后将对所有读者可见。</p>
      </Modal>
    </div>
  );
}
