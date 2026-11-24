import { Alert, Button, Form, Modal, Space, Table, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  getConnections,
  permissionOptions,
  readScopeLabels,
  revokeConnection,
  setPermissions,
} from '../../api/mcp';
import type { McpConnection, McpSettings, McpPermission, McpReadScope } from '../../api/mcp';
import { PermissionPicker } from './PermissionPicker';
import { ReadScopePicker } from './ReadScopePicker';
import { GrantDialog } from './GrantDialog';
import { PublishedUploadPicker } from './PublishedUploadPicker';

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
  const [scope, setScope] = useState<McpReadScope>('own');
  const [uploadAlbums, setUploadAlbums] = useState<string[]>([]);
  function choose(row: McpConnection, type: Action['type']) {
    setError('');
    setSelected(row.permissions);
    setScope(row.readScope);
    setUploadAlbums(row.publishedUploadAlbumIds ?? []);
    setAction({ row, type });
  }
  async function confirm() {
    if (!action) return;
    setBusy(true);
    try {
      if (action.type === 'revoke') await revokeConnection(action.row.id);
      else await setPermissions(action.row.id, selected, scope, action.row.permissionRevision, uploadAlbums);
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
        scroll={{ x: 980 }}
        locale={{ emptyText: '暂无 AI 连接' }}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
        columns={[
          { title: '连接名称', dataIndex: 'label', width: 150 },
          { title: '认证', render: (_, r) => (r.oauth ? 'OAuth' : 'Bearer') },
          {
            title: '客户端',
            render: (_, r) => r.clientName || (r.lastSeenAt ? '未上报名称' : '—'),
          },
          {
            title: '最近访问',
            render: (_, r) =>
              r.lastSeenAt ? new Date(r.lastSeenAt).toLocaleString('zh-CN') : '暂无访问记录',
          },
          {
            title: '数据可见范围',
            render: (_, row) => (row.revoked ? '—' : readScopeLabels[row.readScope]),
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
                      <Tag
                        key={p.value}
                        color={
                          p.value === 'delete_album_photo'
                            ? 'red'
                            : p.value.startsWith('publish_')
                              ? 'gold'
                              : 'blue'
                        }
                      >
                        {p.label}
                      </Tag>
                    ))}
                  {!!r.publishedUploadAlbumIds?.length && (
                    <Tag color="gold">可追加照片 · {r.publishedUploadAlbumIds.length} 个相册</Tag>
                  )}
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
            <>
              <Form.Item label="数据可见范围">
                <ReadScopePicker value={scope} onChange={setScope} />
              </Form.Item>
              <PermissionPicker
                value={selected}
                onChange={(permissions) => {
                  setSelected(permissions);
                  if (!permissions.includes('upload_album_photo')) setUploadAlbums([]);
                }}
              />
              {selected.includes('upload_album_photo') && (
                <Form.Item label="可追加照片的已发布相册" extra="追加的照片立即公开。">
                  <PublishedUploadPicker value={uploadAlbums} onChange={setUploadAlbums} />
                </Form.Item>
              )}
            </>
          )}
          {error && <Alert type="error" showIcon title={error} />}
        </Modal>
      )}
    </Space>
  );
}
