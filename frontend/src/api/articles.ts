import { apiGet, apiPatch, apiPost } from './client';

export type ArticleStatus = 'draft' | 'published' | 'archived';

export interface ArticleFormData {
  title: string;
  slug: string;
  articleName: string;
  summary: string;
  tags: string[];
  bodyMdx: string;
  coverImageUrl: string;
}

export interface AdminArticleRecord extends ArticleFormData {
  articleId: string;
  status: ArticleStatus;
  publishedAt: string | null;
}

export interface AdminArticlePayload {
  articleName: string;
  title: string;
  summary: string;
  bodyMdx: string;
  tags: string[];
  featured: boolean;
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
