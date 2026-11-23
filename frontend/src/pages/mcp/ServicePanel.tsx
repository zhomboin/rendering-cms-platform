import { Alert, App, Button, Card, Form, Input, Modal, Space, Switch, Tag, Typography } from 'antd';
import { useState } from 'react';
import { saveSettings } from '../../api/mcp';
import type { McpSettings, SettingsInput } from '../../api/mcp';

type Editor = Omit<SettingsInput, 'revision'>;

export function ServicePanel({
  settings,
  refresh,
}: {
  settings: McpSettings;
  refresh: () => Promise<unknown>;
}) {
  const { message } = App.useApp();
  const [editing, setEditing] = useState<McpSettings>();
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  async function submit(values: Editor) {
    if (!editing) return;
    setSaving(true);
    setError('');
    try {
      await saveSettings({ ...values, revision: editing.revision });
      await refresh();
      setEditing(undefined);
      void message.success('服务设置已保存');
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
      await refresh();
    } finally {
      setSaving(false);
    }
  }
  return (
    <Card
      title={
        <Space>
          服务
          <Tag color={settings.enabled ? 'green' : 'default'}>{settings.enabled ? '已开启' : '已关闭'}</Tag>
        </Space>
      }
      extra={
        <Button
          onClick={() => {
            setError('');
            setEditing(settings);
          }}
        >
          服务设置
        </Button>
      }
    >
      <Space orientation="vertical">
        <Typography.Text type="secondary">接入地址 · Streamable HTTP</Typography.Text>
        {settings.endpoint ? (
          <Typography.Text copyable style={{ overflowWrap: 'anywhere' }}>
            {settings.endpoint}
          </Typography.Text>
        ) : (
          '待配置'
        )}
      </Space>
      {editing && (
        <Modal
          open
          title="服务设置"
          destroyOnHidden
          okText="保存设置"
          cancelText="取消"
          cancelButtonProps={{ disabled: saving }}
          confirmLoading={saving}
          okButtonProps={{ htmlType: 'submit', form: 'mcp-settings' }}
          onCancel={() => {
            if (!saving) setEditing(undefined);
          }}
        >
          <Form<Editor> id="mcp-settings" layout="vertical" initialValues={editing} onFinish={submit}>
            <Form.Item
              name="enabled"
              label="MCP 服务"
              valuePropName="checked"
              extra="关闭后，所有 AI 连接的后续请求都会停止。"
            >
              <Switch checkedChildren="开启" unCheckedChildren="关闭" />
            </Form.Item>
            <Form.Item
              name="origin"
              label="CMS API 来源"
              rules={[{ required: true, message: '请填写域名和协议' }]}
            >
              <Input placeholder="https://cms.example.com" />
            </Form.Item>
            <Form.Item name="adminOrigin" label="后台来源" extra="与 CMS API 同源时可留空。">
              <Input placeholder="https://admin.example.com" />
            </Form.Item>
            <Form.Item
              name="blogOrigin"
              label="博客来源"
              rules={[{ required: true, message: '请填写博客域名和协议' }]}
            >
              <Input placeholder="https://blog.example.com" />
            </Form.Item>
            {error && <Alert title={error} type="error" showIcon />}
          </Form>
        </Modal>
      )}
    </Card>
  );
}
