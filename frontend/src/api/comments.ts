import { apiGet, apiPatch } from './client';

export type CommentStatus = 'pending' | 'approved' | 'rejected';

export interface AdminComment {
  commentId: string;
  articleId: string;
  articleSlug: string;
  articleTitle: string;
  authorName: string;
  authorEmail: string | null;
  body: string;
  status: CommentStatus;
  userAgent: string | null;
  createdAt: string;
  reviewedAt: string | null;
}

export function listAdminComments() {
  return apiGet<AdminComment[]>('/admin/comments');
}

export function reviewAdminComment(commentId: string, status: CommentStatus) {
  return apiPatch<AdminComment>(`/admin/comments/${commentId}`, { status });
}
