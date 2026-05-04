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

export function getAdminAnalyticsSummary() {
  return apiGet<AnalyticsSummary>('/admin/analytics/summary');
}
