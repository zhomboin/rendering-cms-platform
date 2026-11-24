import { Alert, Button, Modal, Space, Table, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getConnections, permissionOptions, revokeConnection, setPermissions } from '../../api/mcp';
import type { McpConnection, McpSettings, McpPermission } from '../../api/mcp';
import { PermissionPicker } from './PermissionPicker';
import { GrantDialog } from './GrantDialog';

type Action = { row: McpConnection; type: 'permissions' | 'revoke' };
const actions = { permissions: '配置权限', revoke: '撤销连接' };
export function ConnectionsPanel({ owner, settings }: { owner: string; settings: McpSettings }) {
  const connections = useQuery({
    queryKey: ['mcp-connections', owner],
    queryFn: getConnections,
    staleTime: 0,
  });
  const [creating, setCreating] = useState(false);
  const [action, setAction] = useState<Action>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<McpPermission[]>([]);
  function choose(row: McpConnection, type: Action['type']) {
    setError('');
    setSelected(row.permissions);
    setAction({ row, type });
  }
  async function confirm() {
    if (!action) return;
    setBusy(true);
    try {
      if (action.type === 'revoke') await revokeConnection(action.row.id);
      else await setPermissions(action.row.id, selected, action.row.permissionRevision);
      await connections.refetch();
      setAction(undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败');
      void connections.refetch();
    } finally {
      setBusy(false);
    }
  }
  return (
    <Space orientation="vertical" size={12} style={{ display: 'flex' }}>
      <Space wrap>
        <Button type="primary" disabled={!settings.endpoint} onClick={() => setCreating(true)}>
          新建连接
        </Button>
        <Button
          icon={<ReloadOutlined />}
          loading={connections.isFetching}
          onClick={() => void connections.refetch()}
        >
          刷新连接
        </Button>
      </Space>
      {connections.error && <Alert type="error" showIcon title={connections.error.message} />}
      <Table<McpConnection>
        rowKey="id"
        dataSource={connections.data}
        loading={connections.isPending}
        scroll={{ x: 850 }}
        locale={{ emptyText: '暂无 AI 连接' }}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
        columns={[
          { title: '连接名称', dataIndex: 'label', width: 150 },
          { title: '认证', render: (_, r) => (r.oauth ? 'OAuth' : 'Bearer') },
          { title: '客户端', render: (_, r) => r.clientName || (r.lastSeenAt ? '未上报名称' : '—') },
          {
            title: '最近访问',
            render: (_, r) =>
              r.lastSeenAt ? new Date(r.lastSeenAt).toLocaleString('zh-CN') : '暂无访问记录',
          },
          {
            title: '权限',
            width: 240,
            render: (_, r) =>
              r.revoked ? (
                <Tag>已撤销</Tag>
              ) : r.permissions.length === 0 ? (
                '无工具权限'
              ) : (
                <Space size={[2, 4]} wrap>
                  {permissionOptions
                    .filter((p) => r.permissions.includes(p.value))
                    .map((p) => (
                      <Tag key={p.value} color={p.value === 'publish_article' ? 'gold' : 'blue'}>
                        {p.label}
                      </Tag>
                    ))}
                </Space>
              ),
          },
          {
            title: '操作',
            render: (_, r) =>
              r.revoked ? (
                '—'
              ) : (
                <Space wrap>
                  <Button size="small" onClick={() => choose(r, 'permissions')}>
                    配置权限
                  </Button>
                  <Button size="small" danger onClick={() => choose(r, 'revoke')}>
                    撤销
                  </Button>
                </Space>
              ),
          },
        ]}
      />
      {creating && (
        <GrantDialog
          issuer={settings.issuer}
          created={() => void connections.refetch()}
          close={() => setCreating(false)}
        />
      )}
      {action && (
        <Modal
          open
          title={actions[action.type] + ' · ' + action.row.label}
          cancelText="取消"
          okText={action.type === 'permissions' ? '保存权限' : actions.revoke}
          okButtonProps={{ danger: action.type === 'revoke' }}
          confirmLoading={busy}
          onOk={confirm}
          onCancel={() => {
            if (!busy) setAction(undefined);
          }}
        >
          {action.type === 'revoke' ? (
            <p>撤销后，该连接的凭据将永久失效。</p>
          ) : (
            <PermissionPicker value={selected} onChange={setSelected} />
          )}
          {error && <Alert type="error" showIcon title={error} />}
        </Modal>
      )}
    </Space>
  );
}
