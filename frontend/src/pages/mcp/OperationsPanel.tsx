import { Alert, Button, Space, Table, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { getOperations, permissionOptions } from '../../api/mcp';
import type { McpOperation } from '../../api/mcp';

const operationNames: Record<string, string> = {
  ...Object.fromEntries(permissionOptions.map(({ value, label }) => [value, label])),
  unknown: '未知工具',
};
const outcomes = { success: '成功', failed: '失败', pending: '结果待确认' };
export function OperationsPanel({ owner }: { owner: string }) {
  const [cursors, setCursors] = useState(['']);
  const before = cursors[cursors.length - 1];
  const records = useQuery({
    queryKey: ['mcp-operations', owner, before],
    queryFn: () => getOperations(before),
    staleTime: 0,
  });
  const next = records.data?.next;
  return (
    <Space orientation="vertical" size={12} style={{ display: 'flex' }}>
      <Button
        icon={<ReloadOutlined />}
        loading={records.isFetching}
        onClick={() => {
          if (before) setCursors(['']);
          else void records.refetch();
        }}
      >
        刷新记录
      </Button>
      {records.error && <Alert type="error" showIcon title={records.error.message} />}
      <Table<McpOperation>
        rowKey="id"
        dataSource={records.data?.items}
        loading={records.isPending}
        pagination={false}
        locale={{ emptyText: '暂无工具调用记录' }}
        scroll={{ x: 630 }}
        columns={[
          {
            title: '时间',
            dataIndex: 'createdAt',
            render: (v: string) => new Date(v).toLocaleString('zh-CN'),
          },
          { title: '连接', dataIndex: 'connection' },
          { title: '操作', render: (_, row) => operationNames[row.operation] ?? row.operation },
          {
            title: '结果',
            render: (_, row) => (
              <Tag color={row.outcome === 'success' ? 'green' : row.outcome === 'failed' ? 'red' : undefined}>
                {outcomes[row.outcome]}
              </Tag>
            ),
          },
          {
            title: '文章',
            render: (_, row) =>
              row.articleId ? <Link to={'/admin/articles/' + row.articleId + '/read'}>查看文章</Link> : '—',
          },
        ]}
      />
      {(before || next) && (
        <Space>
          <Button disabled={!before} onClick={() => setCursors(cursors.slice(0, -1))}>
            上一页
          </Button>
          <Button
            disabled={!next}
            onClick={() => {
              if (next) setCursors([...cursors, next]);
            }}
          >
            下一页
          </Button>
        </Space>
      )}
    </Space>
  );
}
