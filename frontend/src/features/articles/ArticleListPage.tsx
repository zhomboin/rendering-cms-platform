import { Link } from 'react-router-dom';
import { Card, Typography, Tag, Empty, Skeleton, Alert } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { apiGet } from '../../api/client';

const { Title, Text } = Typography;

interface Article {
  articleId: string;
  title: string;
  slug: string;
  summary: string;
  tags: string[];
  publishedAt: string | null;
}

function formatDate(value: string | null) {
  if (!value) return '未发布';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(value));
}

export default function ArticleListPage() {
  const { data = [], isLoading, error } = useQuery({
    queryKey: ['public-articles'],
    queryFn: () => apiGet<Article[]>('/articles'),
  });

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
      <Title level={3} style={{ fontSize: 24, fontWeight: 700, marginBottom: 24, color: '#0F172A' }}>
        文章
      </Title>

      {error && (
        <Alert
          type="error"
          showIcon
          message={error instanceof Error ? error.message : '文章列表读取失败'}
          style={{ marginBottom: 20 }}
        />
      )}

      {isLoading && <Skeleton active paragraph={{ rows: 8 }} />}

      {!isLoading && data.length === 0 && !error && <Empty description="暂无文章" />}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        {data.map((article) => (
          <Link key={article.slug} to={`/articles/${article.slug}`} style={{ textDecoration: 'none' }}>
            <Card
              hoverable
              style={{
                borderRadius: 8,
                border: '1px solid #E2E8F0',
                boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
              }}
              styles={{ body: { padding: 20 } }}
            >
              <Title level={4} style={{ fontSize: 18, fontWeight: 700, marginBottom: 8, color: '#0F172A' }}>
                {article.title}
              </Title>

              <Text
                style={{
                  fontSize: 14,
                  color: '#64748B',
                  display: 'block',
                  marginBottom: 12,
                  lineHeight: 1.6,
                }}
              >
                {article.summary}
              </Text>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 }}>
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  {article.tags.map((tag) => (
                    <Tag key={tag} style={{ borderRadius: 4, fontSize: 12 }}>
                      {tag}
                    </Tag>
                  ))}
                </div>

                <Text style={{ fontSize: 12, color: '#64748B', flexShrink: 0 }}>
                  {formatDate(article.publishedAt)}
                </Text>
              </div>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
