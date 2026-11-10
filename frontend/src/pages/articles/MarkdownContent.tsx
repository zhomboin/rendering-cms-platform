import { createElement } from 'react';
import type { ComponentProps, ElementType, HTMLAttributes } from 'react';
import { Alert, Skeleton } from 'antd';
import type { ArticleMarkdown } from '../../markdown/engine.js';
import { MarkdownCodeBlock } from './MarkdownCodeBlock';
import './MarkdownContent.css';

function Figure({ src, alt = '', caption }: { src?: string; alt?: string; caption?: string }) {
  if (typeof src !== 'string' || !/^(https?:\/\/|\/(?!\/))/.test(src)) return null;
  return (
    <figure>
      <img src={src} alt={typeof alt === 'string' ? alt : ''} />
      {typeof caption === 'string' && <figcaption>{caption}</figcaption>}
    </figure>
  );
}

function Gallery({
  items = [],
  title,
}: {
  items?: Array<ComponentProps<typeof Figure>>;
  title?: string;
}) {
  return (
    <section aria-label={title}>
      {title && <p>{title}</p>}
      {(Array.isArray(items) ? items : [])
        .filter((item) => item && typeof item === 'object')
        .map((item, index) => (
          <Figure key={index} {...item} />
        ))}
    </section>
  );
}

function Heading({
  level,
  children,
  ...props
}: HTMLAttributes<HTMLHeadingElement> & { level: number }) {
  return createElement(`h${level}`, props, children);
}

const components: Record<string, ElementType> = {
  Figure,
  Gallery,
  pre: MarkdownCodeBlock,
  table: ({ children, ...props }: HTMLAttributes<HTMLTableElement>) => (
    <div className="markdown-table" role="region" aria-label="表格，可横向滚动" tabIndex={0}>
      <table {...props}>{children}</table>
    </div>
  ),
  ...Object.fromEntries(
    [1, 2, 3, 4, 5, 6].map((level) => [
      `h${level}`,
      (props: HTMLAttributes<HTMLHeadingElement>) => <Heading level={level} {...props} />,
    ]),
  ),
};

export function MarkdownContent({
  article,
  error,
  loading,
}: {
  article?: ArticleMarkdown;
  error?: string;
  loading?: boolean;
}) {
  if (error) return <Alert type="error" showIcon title="正文解析失败" description={error} />;
  if (loading) return <Skeleton active paragraph={{ rows: 3 }} />;
  if (!article) return null;
  const Content = article.Content;
  return (
    <div className="markdown-content">
      <Content components={components} />
    </div>
  );
}
