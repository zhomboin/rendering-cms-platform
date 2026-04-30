import { useParams, useNavigate } from 'react-router-dom';
import { Card, Form, Input, Select, Button, Typography, Modal, Space } from 'antd';
import { useState } from 'react';

const { Title } = Typography;
const { TextArea } = Input;

interface ArticleFormData {
  title: string;
  slug: string;
  summary: string;
  tags: string[];
  body: string;
  coverImageUrl: string;
}

const mockExistingArticle: Record<number, ArticleFormData> = {
  1: {
    title: 'Go 语言并发编程实战',
    slug: 'go-concurrency-in-practice',
    summary: '本文深入探讨 Go 语言的并发模型，包括 goroutine、channel 和 sync 包的使用技巧。',
    tags: ['Go', '并发'],
    body: '## 引言\n\nGo 语言以其简洁的并发模型著称...\n\n```go\nfunc main() {\n\tch := make(chan string)\n\tgo func() {\n\t\tch <- "hello"\n\t}()\n\tfmt.Println(<-ch)\n}\n```',
    coverImageUrl: 'https://images.unsplash.com/photo-1516116216624-53e697fedbea',
  },
  2: {
    title: 'React 19 新特性解读',
    slug: 'react-19-new-features',
    summary: 'React 19 带来了许多令人兴奋的新特性，包括 Server Components、Actions 和改进的 Hooks。',
    tags: ['React', '前端'],
    body: '## React 19 概述\n\nReact 19 是 Meta 团队...',
    coverImageUrl: '',
  },
};

const initialFormData: ArticleFormData = {
  title: '',
  slug: '',
  summary: '',
  tags: [],
  body: '',
  coverImageUrl: '',
};

export default function AdminArticleEditorPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [form] = Form.useForm<ArticleFormData>();
  const [publishModalOpen, setPublishModalOpen] = useState(false);

  const isEdit = Boolean(id) && id !== 'new';
  const articleId = isEdit ? Number(id) : undefined;

  const [initialized, setInitialized] = useState(false);
  if (isEdit && articleId && !initialized) {
    const existing = mockExistingArticle[articleId];
    if (existing) {
      form.setFieldsValue(existing);
    }
    setInitialized(true);
  }

  const handleSaveDraft = async () => {
    try {
      const values = await form.validateFields();
      console.log('Save draft', values);
      Modal.success({
        title: '草稿已保存',
        content: '文章草稿已成功保存。',
      });
    } catch {
      // Intentionally empty
    }
  };

  const handlePublish = async () => {
    try {
      const values = await form.validateFields();
      console.log('Publish article', values);
      setPublishModalOpen(false);
      Modal.success({
        title: '文章已发布',
        content: '文章已成功发布。',
        onOk: () => navigate('/admin/articles'),
      });
    } catch {
      setPublishModalOpen(false);
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <Title level={1} style={{ margin: '0 0 24px', fontSize: 24, fontWeight: 700 }}>
        {isEdit ? '编辑文章' : '新建文章'}
      </Title>

      <Card
        style={{
          borderRadius: 24,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px rgba(0,0,0,0.06)',
        }}
        styles={{ body: { padding: 32 } }}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={initialFormData}
          style={{ maxWidth: 800 }}
        >
          <Form.Item
            name="title"
            label="标题"
            rules={[{ required: true, message: '请输入文章标题' }]}
          >
            <Input placeholder="输入文章标题" size="large" />
          </Form.Item>

          <Form.Item
            name="slug"
            label="Slug"
            rules={[{ required: true, message: '请输入 URL Slug' }]}
          >
            <Input
              addonBefore="/articles/"
              placeholder="my-article-slug"
              size="large"
            />
          </Form.Item>

          <Form.Item name="summary" label="摘要">
            <TextArea
              rows={3}
              placeholder="文章摘要（可选）"
              showCount
              maxLength={300}
            />
          </Form.Item>

          <Form.Item name="tags" label="标签">
            <Select
              mode="tags"
              placeholder="输入标签后按回车添加"
              style={{ width: '100%' }}
              tokenSeparators={[',', '，']}
            />
          </Form.Item>

          <Form.Item
            name="body"
            label="正文"
            rules={[{ required: true, message: '请输入文章正文' }]}
          >
            <TextArea
              rows={12}
              placeholder="使用 Markdown 格式编写文章正文..."
              style={{ fontFamily: "'Fira Code', 'JetBrains Mono', 'SF Mono', Consolas, monospace" }}
            />
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
        <Button onClick={() => navigate('/admin/articles')}>
          取消
        </Button>
        <Space size={12}>
          <Button
            onClick={handleSaveDraft}
            style={{
              borderColor: '#4F46E5',
              color: '#4F46E5',
            }}
          >
            保存草稿
          </Button>
          <Button
            type="primary"
            size="large"
            style={{
              backgroundColor: '#4F46E5',
              borderColor: '#4F46E5',
              fontWeight: 600,
            }}
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
        okButtonProps={{
          style: {
            backgroundColor: '#4F46E5',
            borderColor: '#4F46E5',
          },
        }}
      >
        <p>确定要发布这篇文章吗？发布后将对所有读者可见。</p>
      </Modal>
    </div>
  );
}
