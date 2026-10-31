import { useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Alert, Button, Empty, Skeleton, Space, Tag } from 'antd';
import { ArrowLeftOutlined, EditOutlined, CalendarOutlined, HistoryOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { listAdminArticles } from '../../api/articles';
import type { ArticleStatus } from '../../api/articles';
import { MdxBlockRenderer, collectHeadingAnchors } from './MdxBlockRenderer';
import { parsePreviewBlocks } from './MdxPreviewParser';
import './ArticleReadingPage.css';

const statusConfig: Record<ArticleStatus, { color: string; label: string }> = {
  draft: { color: 'default', label: '草稿' },
  published: { color: 'success', label: '已发布' },
  private: { color: 'purple', label: '私有' },
  archived: { color: 'warning', label: '已归档' },
};

function formatDateTime(value: string | null | undefined) {
  if (!value) return null;
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export default function ArticleReadingPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const articlesQuery = useQuery({
    queryKey: ['admin-articles'],
    queryFn: listAdminArticles,
    enabled: Boolean(id),
  });

  const article = useMemo(
    () => articlesQuery.data?.find((item) => item.articleId === id),
    [articlesQuery.data, id],
  );

  const blocks = useMemo(() => parsePreviewBlocks(article?.bodyMdx ?? ''), [article?.bodyMdx]);
  const headings = useMemo(
    () => collectHeadingAnchors(blocks, id ?? 'article'),
    [blocks, id],
  );

  const handleTocClick = (anchorId: string) => (event: React.MouseEvent<HTMLAnchorElement>) => {
    event.preventDefault();
    document.getElementById(anchorId)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  const statusCfg = article ? statusConfig[article.status] : null;
  const updatedAtText = formatDateTime(article?.updatedAt);
  const publishedAtText = formatDateTime(article?.publishedAt);

  return (
    <div className="article-reading">
      <header className="article-reading__topbar">
        <Button
          type="text"
          icon={<ArrowLeftOutlined />}
          onClick={() => navigate('/admin/articles')}
        >
          返回文章列表
        </Button>
        {article && statusCfg && (
          <div className="article-reading__topbar-meta">
            <Tag color={statusCfg.color}>{statusCfg.label}</Tag>
            <Button
              type="primary"
              icon={<EditOutlined />}
              style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5' }}
              onClick={() => navigate(`/admin/articles/${article.articleId}/edit`)}
            >
              编辑
            </Button>
          </div>
        )}
      </header>

      {articlesQuery.isLoading && (
        <div style={{ maxWidth: 760, margin: '48px auto', padding: '0 24px' }}>
          <Skeleton active paragraph={{ rows: 12 }} />
        </div>
      )}

      {articlesQuery.error && (
        <div style={{ maxWidth: 760, margin: '48px auto', padding: '0 24px' }}>
          <Alert
            type="error"
            showIcon
            message={articlesQuery.error instanceof Error ? articlesQuery.error.message : '文章读取失败'}
            action={
              <Button size="small" onClick={() => articlesQuery.refetch()}>
                重试
              </Button>
            }
          />
        </div>
      )}

      {!articlesQuery.isLoading && !articlesQuery.error && !article && (
        <div className="article-reading__empty">
          <Empty description="文章不存在或已被删除">
            <Button onClick={() => navigate('/admin/articles')}>返回文章列表</Button>
          </Empty>
        </div>
      )}

      {article && (
        <div className="article-reading__layout">
          <main className="article-reading__content">
            <header className="article-reading__header">
              {statusCfg && <Tag color={statusCfg.color}>{statusCfg.label}</Tag>}
              <h1 className="article-reading__title">{article.title}</h1>
              {article.summary && <p className="article-reading__summary">{article.summary}</p>}
              <div className="article-reading__meta">
                {article.publishedAt && publishedAtText && (
                  <span className="article-reading__meta-item">
                    <CalendarOutlined />
                    发布于 {publishedAtText}
                  </span>
                )}
                {updatedAtText && (
                  <span className="article-reading__meta-item">
                    <HistoryOutlined />
                    更新于 {updatedAtText}
                  </span>
                )}
                {typeof article.version === 'number' && <span>v{article.version}</span>}
                {article.tags.length > 0 && (
                  <Space size={4} wrap>
                    {article.tags.map((tag) => (
                      <Tag key={tag} style={{ marginRight: 0 }}>
                        {tag}
                      </Tag>
                    ))}
                  </Space>
                )}
              </div>
            </header>

            <div className="article-reading__body">
              {blocks.length === 0 ? (
                <Empty description="暂无正文内容" image={Empty.PRESENTED_IMAGE_SIMPLE} />
              ) : (
                <MdxBlockRenderer blocks={blocks} variant="reading" headingIdPrefix={id ?? 'article'} />
              )}
            </div>
          </main>

          {headings.length > 0 && (
            <nav className="article-reading__toc" aria-label="文章目录">
              <div className="article-reading__toc-title">目录</div>
              {headings.map((heading) => (
                <a
                  key={heading.id}
                  href={`#${heading.id}`}
                  data-level={heading.level}
                  onClick={handleTocClick(heading.id)}
                >
                  {heading.text}
                </a>
              ))}
            </nav>
          )}
        </div>
      )}
    </div>
  );
}
