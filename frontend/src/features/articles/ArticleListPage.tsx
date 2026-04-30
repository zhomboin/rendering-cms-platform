import { useNavigate } from 'react-router-dom';
import { Card, Typography, Tag, Empty } from 'antd';

const { Title, Text } = Typography;

interface Article {
  title: string;
  slug: string;
  summary: string;
  tags: string[];
  publishedAt: string;
}

const mockArticles: Article[] = [
  {
    title: '深入理解Go并发模型',
    slug: 'understanding-go-concurrency',
    summary:
      'Go 语言的 goroutine 和 channel 是其并发编程的核心。本文从运行时调度出发，深入剖析 GMP 模型、协作式抢占和 channel 通信机制，帮助读者建立完整的并发知识体系。',
    tags: ['Go', '并发'],
    publishedAt: '2026-04-28',
  },
  {
    title: 'React 19 新特性解析',
    slug: 'react-19-new-features',
    summary:
      'React 19 带来了 Action、Server Components、新 Hook 等多项重磅更新。本文结合实际场景，逐一解读这些新特性的设计思路与使用方式。',
    tags: ['React', '前端'],
    publishedAt: '2026-04-25',
  },
  {
    title: '使用TailwindCSS构建现代UI',
    slug: 'building-modern-ui-tailwindcss',
    summary:
      'TailwindCSS 的 Utility-First 理念正在改变前端开发方式。本文从设计系统映射、响应式策略和性能优化三个角度，分享在实际项目中的最佳实践。',
    tags: ['CSS', '设计'],
    publishedAt: '2026-04-20',
  },
];

export default function ArticleListPage() {
  const navigate = useNavigate();

  if (mockArticles.length === 0) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
        <Title
          level={3}
          style={{ fontSize: 24, fontWeight: 700, marginBottom: 24 }}
        >
          文章
        </Title>
        <Empty description="暂无文章" />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
      <Title
        level={3}
        style={{ fontSize: 24, fontWeight: 700, marginBottom: 24, color: '#0F172A' }}
      >
        文章
      </Title>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        {mockArticles.map((article) => (
          <Card
            key={article.slug}
            hoverable
            onClick={() => navigate(`/articles/${article.slug}`)}
            style={{
              borderRadius: 24,
              border: '1px solid #E2E8F0',
              boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
              transition: 'box-shadow 200ms ease-out',
            }}
            styles={{
              body: { padding: 20 },
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.boxShadow =
                '0 4px 16px rgba(0, 0, 0, 0.08)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.boxShadow =
                '0 1px 3px rgba(0, 0, 0, 0.04)';
            }}
          >
            <Title
              level={4}
              style={{ fontSize: 18, fontWeight: 700, marginBottom: 8, color: '#0F172A' }}
            >
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

            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <div style={{ display: 'flex', gap: 6 }}>
                {article.tags.map((tag) => (
                  <Tag key={tag} style={{ borderRadius: 4, fontSize: 12 }}>
                    {tag}
                  </Tag>
                ))}
              </div>

              <Text
                style={{ fontSize: 12, color: '#64748B' }}
              >
                {article.publishedAt}
              </Text>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
