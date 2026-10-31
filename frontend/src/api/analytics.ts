import { apiGet } from './client';

export interface HotArticle {
  rank: number;
  title: string;
  views: number;
  slug: string;
}

export interface DailyView {
  date: string;
  views: number;
}

export interface AnalyticsSummary {
  todayViews: number;
  last7Days: DailyView[];
  hotArticles: HotArticle[];
}

export interface ArticleAnalytics {
  title: string;
  slug: string;
  todayViews: number;
  periodViews: number;
  totalViews: number;
  publishedAt: string | null;
}

export interface ArticleAnalyticsResponse {
  days: number;
  articles: ArticleAnalytics[];
}

export function getAdminAnalyticsSummary() {
  return apiGet<AnalyticsSummary>('/admin/analytics/summary');
}

export function getAdminArticleAnalytics(days = 7) {
  return apiGet<ArticleAnalyticsResponse>(`/admin/analytics/articles?days=${days}`);
}

export interface AnalyticsTrend {
  days: number;
  site: DailyView[];
  articles: { date: string; slug: string; title: string; views: number }[];
}

export function getAdminAnalyticsTrend(days: number) {
  return apiGet<AnalyticsTrend>('/admin/analytics/trend?days=' + days);
}
