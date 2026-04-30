import { useParams, Link } from 'react-router-dom';
import { Card, Typography, Tag, Input, Button, Empty, Divider } from 'antd';

const { Title, Text } = Typography;
const { TextArea } = Input;

interface Article {
  title: string;
  slug: string;
  summary: string;
  body: string;
  tags: string[];
  publishedAt: string;
}

interface Comment {
  id: number;
  author: string;
  content: string;
  createdAt: string;
}

const mockArticles: Record<string, Article> = {
  'understanding-go-concurrency': {
    title: '深入理解Go并发模型',
    slug: 'understanding-go-concurrency',
    summary:
      'Go 语言的 goroutine 和 channel 是其并发编程的核心。本文从运行时调度出发，深入剖析 GMP 模型、协作式抢占和 channel 通信机制，帮助读者建立完整的并发知识体系。',
    body: `Go 语言自诞生之初就将并发作为一等公民。与其他语言基于线程的并发模型不同，Go 提供了 goroutine 和 channel 两种原语，极大地降低了并发编程的复杂度。

## GMP 调度模型

Go 的运行时调度器采用 GMP 模型，其中 G 代表 goroutine，M 代表操作系统线程，P 代表逻辑处理器。每个 P 维护一个本地 goroutine 队列，当某个 M 绑定的 P 队列为空时，它会从其他 P 或全局队列中窃取 goroutine 来执行。

这种设计使得 Go 能够以极少的 OS 线程承载数以万计的 goroutine，调度开销远低于传统线程切换。

## Channel 通信机制

Channel 是 Go 中 goroutine 之间通信的主要方式。根据是否带缓冲区，channel 分为同步和异步两种。无缓冲 channel 保证发送和接收同时发生，实现了严格的同步；有缓冲 channel 则允许一定程度的解耦。

Go 的设计哲学是"不要通过共享内存来通信，而应该通过通信来共享内存"，channel 正是这一理念的具体体现。

## 协作式抢占

Go 1.14 引入了基于信号的协作式抢占机制，解决了之前版本中循环计算无法被抢占的问题。现在运行时调度器可以更公平地分配 CPU 时间，防止某个 goroutine 长时间占用 P 而导致其他 goroutine 饥饿。`,
    tags: ['Go', '并发'],
    publishedAt: '2026-04-28',
  },
  'react-19-new-features': {
    title: 'React 19 新特性解析',
    slug: 'react-19-new-features',
    summary:
      'React 19 带来了 Action、Server Components、新 Hook 等多项重磅更新。本文结合实际场景，逐一解读这些新特性的设计思路与使用方式。',
    body: `React 19 是 Meta 团队在框架演进道路上的一个重要里程碑。本次更新聚焦于提升开发者体验和应用性能。

## Actions

Actions 是 React 19 中最引人注目的新特性之一。它允许开发者以声明式的方式处理表单提交、数据变更和状态更新，自动管理 pending、error 和 optimistic 状态。

## Server Components

虽然 Server Components 在 React 18 中就已实验性引入，但 React 19 将其作为一等公民正式推出。Server Components 可以在服务端渲染，减少客户端 JavaScript 体积，提升首屏加载性能。

## 新 Hook

React 19 引入了 use() 和 useOptimistic() 等新 Hook，进一步简化了数据获取和乐观更新的代码模式。`,
    tags: ['React', '前端'],
    publishedAt: '2026-04-25',
  },
  'building-modern-ui-tailwindcss': {
    title: '使用TailwindCSS构建现代UI',
    slug: 'building-modern-ui-tailwindcss',
    summary:
      'TailwindCSS 的 Utility-First 理念正在改变前端开发方式。本文从设计系统映射、响应式策略和性能优化三个角度，分享在实际项目中的最佳实践。',
    body: `TailwindCSS 通过提供原子化的 CSS 工具类，让开发者无需离开 HTML 就能构建复杂的用户界面。这种开发方式在提升效率的同时，也带来了良好的可维护性。

## 设计系统映射

将设计 Token 映射到 TailwindCSS 配置是项目初始化的关键步骤。通过扩展主题配置，可以确保颜色、间距、字体等设计规范在整个项目中保持一致。

## 响应式策略

TailwindCSS 的响应式前缀机制（sm:, md:, lg: 等）让断点管理变得直观。采用 Mobile-First 的设计思路，从最小屏幕开始构建，逐步增强大屏体验。

## 性能优化

在生产构建中，TailwindCSS 通过 PurgeCSS 移除未使用的样式，将最终 CSS 体积控制在极小范围内。配合 JIT 引擎，开发环境的热重载速度也得到了显著提升。`,
    tags: ['CSS', '设计'],
    publishedAt: '2026-04-20',
  },
};

const placeholderComments: Comment[] = [];

export default function ArticleDetailPage() {
  const { slug } = useParams<{ slug: string }>();

  const article = slug ? mockArticles[slug] : undefined;

  if (!article) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
        <Link
          to="/articles"
          style={{
            display: 'inline-block',
            marginBottom: 24,
            color: '#4F46E5',
            fontSize: 14,
            textDecoration: 'none',
          }}
        >
          ← 返回文章列表
        </Link>
        <Empty description="文章未找到" />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '48px 24px' }}>
      <Link
        to="/articles"
        style={{
          display: 'inline-block',
          marginBottom: 24,
          color: '#4F46E5',
          fontSize: 14,
          textDecoration: 'none',
        }}
      >
        ← 返回文章列表
      </Link>

      <Title
        level={3}
        style={{ fontSize: 24, fontWeight: 700, marginBottom: 12, color: '#0F172A' }}
      >
        {article.title}
      </Title>

      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 32,
        }}
      >
        <div style={{ display: 'flex', gap: 6 }}>
          {article.tags.map((tag) => (
            <Tag key={tag} style={{ borderRadius: 4, fontSize: 12 }}>
              {tag}
            </Tag>
          ))}
        </div>
        <Text style={{ fontSize: 12, color: '#64748B' }}>
          {article.publishedAt}
        </Text>
      </div>

      <Card
        style={{
          borderRadius: 24,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
          marginBottom: 32,
        }}
        styles={{ body: { padding: 32 } }}
      >
        <div
          style={{
            fontSize: 14,
            lineHeight: 1.8,
            color: '#0F172A',
            whiteSpace: 'pre-wrap',
          }}
        >
          {article.body}
        </div>
      </Card>

      <Divider style={{ borderColor: '#E2E8F0', margin: '0 0 24px 0' }} />

      <Title
        level={4}
        style={{ fontSize: 18, fontWeight: 700, marginBottom: 20, color: '#0F172A' }}
      >
        评论
      </Title>

      {placeholderComments.length === 0 ? (
        <Empty
          description="暂无评论"
          style={{ marginBottom: 24 }}
        />
      ) : (
        <div style={{ marginBottom: 24 }}>
          {placeholderComments.map((comment) => (
            <Card
              key={comment.id}
              style={{
                borderRadius: 16,
                border: '1px solid #E2E8F0',
                marginBottom: 12,
              }}
              styles={{ body: { padding: 16 } }}
            >
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  marginBottom: 8,
                }}
              >
                <Text strong style={{ fontSize: 14 }}>
                  {comment.author}
                </Text>
                <Text style={{ fontSize: 12, color: '#64748B' }}>
                  {comment.createdAt}
                </Text>
              </div>
              <Text style={{ fontSize: 14, color: '#0F172A' }}>
                {comment.content}
              </Text>
            </Card>
          ))}
        </div>
      )}

      <Card
        style={{
          borderRadius: 24,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
        }}
        styles={{ body: { padding: 24 } }}
      >
        <Text strong style={{ fontSize: 14, marginBottom: 16, display: 'block' }}>
          发表评论
        </Text>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Input
            placeholder="您的昵称"
            style={{ height: 40, borderRadius: 8, maxWidth: 300 }}
          />
          <TextArea
            placeholder="写下您的评论..."
            rows={4}
            style={{ borderRadius: 8 }}
          />
          <Button
            type="primary"
            style={{
              alignSelf: 'flex-end',
              height: 40,
              borderRadius: 8,
              backgroundColor: '#4F46E5',
            }}
          >
            提交评论
          </Button>
        </div>
      </Card>
    </div>
  );
}
