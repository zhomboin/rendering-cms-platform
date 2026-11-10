import type { ComponentType, ElementType } from 'react';
export type Heading = { id: string; heading: string; level: 1 | 2 | 3 | 4 | 5 | 6 };
export type ArticleMarkdown = {
  content: string;
  headings: Heading[];
  Content: ComponentType<{ components?: Record<string, ElementType> }>;
};
export const MARKDOWN_CONTRACT: string;
export function parseArticleSource(source?: string): Pick<ArticleMarkdown, 'content' | 'headings'>;
export function compileArticle(source: string): Promise<ArticleMarkdown>;
