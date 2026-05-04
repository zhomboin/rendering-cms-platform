import { useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Card, Typography, Tag, Input, Button, Empty, Divider, Skeleton, Alert, Form, message } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiGet, apiPost } from '../../api/client';

const { Title, Text } = Typography;
const { TextArea } = Input;

interface Article {
  articleId: string;
  title: string;
  slug: string;
  summary: string;
  bodyMdx: string;
  tags: string[];
  publishedAt: string | null;
}

interface Comment {
  commentId: string;
  authorName: string;
  body: string;
  createdAt: string;
}

interface CommentFormValues {
  authorName: string;
  authorEmail?: string;
  body: string;
}

function formatDate(value: string | null) {
  if (!value) return '';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export default function ArticleDetailPage() {
  const { slug } = useParams<{ slug: string }>();
  const [form] = Form.useForm<CommentFormValues>();
  const queryClient = useQueryClient();

  const articleQuery = useQuery({
    queryKey: ['public-article', slug],
    enabled: Boolean(slug),
    queryFn: () => apiGet<Article>(`/articles/${slug}`),
  });

  const commentsQuery = useQuery({
    queryKey: ['public-comments', slug],
    enabled: Boolean(slug),
    queryFn: () => apiGet<Comment[]>(`/articles/${slug}/comments`),
  });

  useEffect(() => {
    if (!slug || !articleQuery.data) return;
    void apiPost(`/articles/${slug}/views`);
  }, [slug, articleQuery.data]);

  const submitComment = useMutation({
    mutationFn: (values: CommentFormValues) => apiPost(`/articles/${slug}/comments`, values),
    onSuccess: async () => {
      message.success('评论已提交，审核通过后会公开展示');
      form.resetFields();
      await queryClient.invalidateQueries({ queryKey: ['public-comments', slug] });
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '评论提交失败');
    },
  });

  if (articleQuery.isLoading) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
        <Skeleton active paragraph={{ rows: 12 }} />
      </div>
    );
  }

  if (!articleQuery.data) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
        <Link to="/articles" style={{ display: 'inline-block', marginBottom: 24, color: '#4F46E5', fontSize: 14 }}>
          返回文章列表
        </Link>
        <Empty description={articleQuery.error instanceof Error ? articleQuery.error.message : '文章未找到'} />
      </div>
    );
  }

  const article = articleQuery.data;
  const comments = commentsQuery.data ?? [];

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
      <Link to="/articles" style={{ display: 'inline-block', marginBottom: 24, color: '#4F46E5', fontSize: 14 }}>
        返回文章列表
      </Link>

      <Title level={3} style={{ fontSize: 24, fontWeight: 700, marginBottom: 12, color: '#0F172A' }}>
        {article.title}
      </Title>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 32, gap: 12 }}>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {article.tags.map((tag) => (
            <Tag key={tag} style={{ borderRadius: 4, fontSize: 12 }}>
              {tag}
            </Tag>
          ))}
        </div>
        <Text style={{ fontSize: 12, color: '#64748B', flexShrink: 0 }}>{formatDate(article.publishedAt)}</Text>
      </div>

      <Card
        style={{
          borderRadius: 8,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
          marginBottom: 32,
        }}
        styles={{ body: { padding: 32 } }}
      >
        <div style={{ fontSize: 14, lineHeight: 1.8, color: '#0F172A', whiteSpace: 'pre-wrap' }}>
          {article.bodyMdx}
        </div>
      </Card>

      <Divider style={{ borderColor: '#E2E8F0', margin: '0 0 24px 0' }} />

      <Title level={4} style={{ fontSize: 18, fontWeight: 700, marginBottom: 20, color: '#0F172A' }}>
        评论
      </Title>

      {commentsQuery.error && (
        <Alert
          type="error"
          showIcon
          message={commentsQuery.error instanceof Error ? commentsQuery.error.message : '评论读取失败'}
          style={{ marginBottom: 16 }}
        />
      )}

      {comments.length === 0 ? (
        <Empty description="暂无评论" style={{ marginBottom: 24 }} />
      ) : (
        <div style={{ marginBottom: 24 }}>
          {comments.map((comment) => (
            <Card
              key={comment.commentId}
              style={{ borderRadius: 8, border: '1px solid #E2E8F0', marginBottom: 12 }}
              styles={{ body: { padding: 16 } }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8, gap: 12 }}>
                <Text strong style={{ fontSize: 14 }}>
                  {comment.authorName}
                </Text>
                <Text style={{ fontSize: 12, color: '#64748B' }}>{formatDate(comment.createdAt)}</Text>
              </div>
              <Text style={{ fontSize: 14, color: '#0F172A' }}>{comment.body}</Text>
            </Card>
          ))}
        </div>
      )}

      <Card style={{ borderRadius: 8, border: '1px solid #E2E8F0', boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)' }} styles={{ body: { padding: 24 } }}>
        <Text strong style={{ fontSize: 14, marginBottom: 16, display: 'block' }}>
          发表评论
        </Text>

        <Form form={form} layout="vertical" onFinish={(values) => submitComment.mutate(values)}>
          <Form.Item name="authorName" rules={[{ required: true, message: '请输入昵称' }]}>
            <Input placeholder="您的昵称" style={{ height: 40, borderRadius: 8, maxWidth: 300 }} />
          </Form.Item>
          <Form.Item name="authorEmail">
            <Input placeholder="邮箱（可选）" style={{ height: 40, borderRadius: 8, maxWidth: 300 }} />
          </Form.Item>
          <Form.Item name="body" rules={[{ required: true, message: '请输入评论内容' }]}>
            <TextArea placeholder="写下您的评论..." rows={4} style={{ borderRadius: 8 }} />
          </Form.Item>
          <Button
            type="primary"
            htmlType="submit"
            loading={submitComment.isPending}
            style={{ height: 40, borderRadius: 8, backgroundColor: '#4F46E5' }}
          >
            提交评论
          </Button>
        </Form>
      </Card>
    </div>
  );
}
