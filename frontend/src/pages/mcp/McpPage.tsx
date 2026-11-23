import { Alert, Button, Space, Spin, Tabs, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { Navigate } from 'react-router-dom';
import { getAuthUser } from '../../api/auth';
import { getSettings } from '../../api/mcp';
import { ServicePanel } from './ServicePanel';
import { ConnectionsPanel } from './ConnectionsPanel';
import { OperationsPanel } from './OperationsPanel';

function Workspace({ owner }: { owner: string }) {
  const settings = useQuery({ queryKey: ['mcp-settings', owner], queryFn: getSettings, staleTime: 0 });
  return (
    <section style={{ minWidth: 0 }}>
      <Typography.Title level={1} style={{ fontSize: 24, marginTop: 0 }}>
        MCP 管理
      </Typography.Title>
      {settings.isPending && <Spin />}
      {settings.error && (
        <Alert
          type="error"
          showIcon
          title={settings.error.message}
          action={<Button onClick={() => void settings.refetch()}>重试</Button>}
        />
      )}
      {settings.data && (
        <Space orientation="vertical" size={16} style={{ display: 'flex' }}>
          <ServicePanel settings={settings.data} refresh={() => settings.refetch()} />
          <Tabs
            items={[
              {
                key: 'connections',
                label: 'AI 连接',
                children: <ConnectionsPanel owner={owner} settings={settings.data} />,
              },
              { key: 'operations', label: '操作记录', children: <OperationsPanel owner={owner} /> },
            ]}
          />
        </Space>
      )}
    </section>
  );
}
export default function McpPage() {
  const user = getAuthUser();
  if (user?.role !== 'admin') return <Navigate to="/admin/articles" replace />;
  return <Workspace key={user.userId} owner={user.userId} />;
}
