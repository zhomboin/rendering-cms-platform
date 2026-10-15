import { apiGet, apiPatch, apiPost } from './client';

export type ArticleStatus = 'draft' | 'published' | 'archived';

export interface ArticleFormData {
  title: string;
  slug: string;
  articleName: string;
  summary: string;
  tags: string[];
  bodyMdx: string;
  featured: boolean;
  featuredRank: number;
  featuredAt: string | null;
  coverImageUrl: string;
}

export interface AdminArticleRecord extends ArticleFormData {
  articleId: string;
  status: ArticleStatus;
  publishedAt: string | null;
  version?: number;
}

export interface AdminArticlePayload {
  articleName: string;
  title: string;
  summary: string;
  bodyMdx: string;
  tags: string[];
  featured: boolean;
  featuredRank: number;
  featuredAt: string | null;
  coverImageUrl: string;
}

export function listAdminArticles() {
  return apiGet<AdminArticleRecord[]>('/admin/articles');
}

export function createAdminArticle(payload: AdminArticlePayload) {
  return apiPost<AdminArticleRecord>('/admin/articles', payload);
}

export function updateAdminArticle(articleId: string, payload: AdminArticlePayload) {
  return apiPatch<AdminArticleRecord>(`/admin/articles/${articleId}`, payload);
}

export function publishAdminArticle(articleId: string) {
  return apiPost<AdminArticleRecord>(`/admin/articles/${articleId}/publish`);
}

export function unpublishAdminArticle(articleId: string) {
  return apiPost<AdminArticleRecord>(`/admin/articles/${articleId}/unpublish`);
}

export interface ArticleRevisionSummary {
  articleId: string;
  slug: string;
  articleName: string;
  title: string;
  summary: string;
  status: ArticleStatus;
  tags: string[];
  featured: boolean;
  featuredRank: number;
  featuredAt: string | null;
  coverImageUrl: string | null;
  publishedAt: string | null;
  version: number;
  updatedAt: string | null;
}

export interface ArticleRevisionDetail extends ArticleRevisionSummary {
  bodyMdx: string;
}

export function listArticleRevisions(articleId: string) {
  return apiGet<ArticleRevisionSummary[]>(`/admin/articles/${articleId}/revisions`);
}

export function getArticleRevision(articleId: string, version: number) {
  return apiGet<ArticleRevisionDetail>(`/admin/articles/${articleId}/revisions/${version}`);
}
