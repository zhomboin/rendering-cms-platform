import type { ArticleStatus } from '../../api/dashboard';

export const number = (value: number) => new Intl.NumberFormat('zh-CN').format(value);

export function date(value: string | null) {
  if (!value) return '尚未发布';
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit' }).format(new Date(value));
}

export function timestamp(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(new Date(value));
}

export function bytes(value: number) {
  if (value < 1024) return number(value) + ' B';
  if (value < 1024 * 1024) return (value / 1024).toFixed(1) + ' KB';
  if (value < 1024 * 1024 * 1024) return (value / (1024 * 1024)).toFixed(1) + ' MB';
  return (value / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
}

export const statusLabel: Record<ArticleStatus, string> = {
  published: '已发布', draft: '草稿', private: '私有 · 仅后台', archived: '已归档',
};
