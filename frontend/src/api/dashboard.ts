import { apiGet } from './client';

export type ArticleStatus = 'draft' | 'published' | 'private' | 'archived';

export interface DashboardArticle {
  articleId: string;
  slug: string;
  title: string;
  status: ArticleStatus;
  coverImageUrl: string;
  tags: string[];
  updatedAt: string;
  publishedAt: string | null;
}

export interface DashboardOverview {
  days: number;
  generatedAt: string;
  startDate: string;
  endDate: string;
  metrics: {
    todayViews: number;
    periodViews: number;
    articleViews: number;
    albumViews: number;
    newComments: number;
    pendingComments: number;
  };
  content: {
    publishedArticles: number;
    draftArticles: number;
    privateArticles: number;
    archivedArticles: number;
    publishedAlbums: number;
    draftAlbums: number;
    photoCount: number;
    missingCoverArticles: number;
  };
  resources: { assetCount: number; imageCount: number; totalBytes: number };
  recentArticles: DashboardArticle[];
  drafts: DashboardArticle[];
  pendingComments: {
    commentId: string;
    articleId: string;
    articleTitle: string;
    authorName: string;
    body: string;
    createdAt: string;
  }[];
  recentImages: {
    assetId: string;
    filename: string;
    publicUrl: string;
    byteSize: number;
    createdAt: string;
  }[];
}

export interface DashboardArticles {
  days: number;
  articles: {
    articleId: string;
    slug: string;
    title: string;
    coverImageUrl: string;
    tags: string[];
    periodViews: number;
    totalViews: number;
    publishedAt: string | null;
  }[];
}

export interface DashboardAlbums {
  days: number;
  albums: {
    albumId: string;
    slug: string;
    title: string;
    status: 'draft' | 'published';
    coverUrl: string;
    photoCount: number;
    periodViews: number;
    updatedAt: string;
  }[];
}

export function getDashboardOverview(days: number) {
  return apiGet<DashboardOverview>('/admin/dashboard/overview?days=' + days);
}
export function getDashboardArticles(days: number) {
  return apiGet<DashboardArticles>('/admin/dashboard/articles?days=' + days);
}
export function getDashboardAlbums(days: number) {
  return apiGet<DashboardAlbums>('/admin/dashboard/albums?days=' + days);
}
