import { apiGet } from './client';

export interface DailyView {
  date: string;
  views: number;
}

export interface AnalyticsTrend {
  days: number;
  site: DailyView[];
  articles: { date: string; slug: string; title: string; views: number }[];
}

export function getAdminAnalyticsTrend(days: number) {
  return apiGet<AnalyticsTrend>('/admin/analytics/trend?days=' + days);
}
