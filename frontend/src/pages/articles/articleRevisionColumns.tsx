import { Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { ArticleRevisionSummary } from '../../api/articles';

const revisionStatusConfig: Record<string, { color: string; label: string }> = {
  draft: { color: 'default', label: '草稿' },
  published: { color: 'success', label: '已发布' },
  private: { color: 'purple', label: '私有' },
  archived: { color: 'warning', label: '已归档' },
};

export function formatRevisionTime(value: string | null) {
  if (!value) return '-';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export function revisionListColumns(currentVersion?: number): ColumnsType<ArticleRevisionSummary> {
  return [
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 110,
      render: (version: number) =>
        version === currentVersion ? <Tag color="processing">v{version} 当前</Tag> : `v${version}`,
    },
    { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (status: string) => {
        const config = revisionStatusConfig[status];
        return <Tag color={config?.color}>{config?.label ?? status}</Tag>;
      },
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 150,
      render: (value: string | null) => formatRevisionTime(value),
    },
  ];
}
