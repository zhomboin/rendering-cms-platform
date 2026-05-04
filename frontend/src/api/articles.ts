import { apiGet, apiPatch, apiPost } from './client';

export type ArticleStatus = 'draft' | 'published' | 'archived';

export interface PublicArticle {
  articleId: string;
  title: string;
  slug: string;
  summary: string;
  tags: string[];
  publishedAt: string | null;
}

export interface PublicArticleDetail extends PublicArticle {
  bodyMdx: string;
}

export interface PublicComment {
  commentId: string;
  authorName: string;
  body: string;
  createdAt: string;
}

export interface CommentFormValues {
  authorName: string;
  authorEmail?: string;
  body: string;
}

export interface ArticleFormData {
  title: string;
  slug: string;
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
  slug: string;
  title: string;
  summary: string;
  bodyMdx: string;
  tags: string[];
  featured: boolean;
  coverImageUrl: string;
}

export function listPublicArticles() {
  return apiGet<PublicArticle[]>('/articles');
}

export function getPublicArticle(slug: string) {
  return apiGet<PublicArticleDetail>(`/articles/${slug}`);
}

export function listPublicArticleComments(slug: string) {
  return apiGet<PublicComment[]>(`/articles/${slug}/comments`);
}

export function recordArticleView(slug: string) {
  return apiPost(`/articles/${slug}/views`);
}

export function submitPublicArticleComment(slug: string, values: CommentFormValues) {
  return apiPost(`/articles/${slug}/comments`, values);
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
