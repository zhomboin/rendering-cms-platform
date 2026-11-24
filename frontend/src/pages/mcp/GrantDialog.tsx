import { Alert, Button, Form, Input, Modal, Select, Space, Typography } from 'antd';
import { useState } from 'react';
import { createConnection, defaultPermissions } from '../../api/mcp';
import { PermissionPicker } from './PermissionPicker';
import { ReadScopePicker } from './ReadScopePicker';
import type { GrantInput, GrantResult } from '../../api/mcp';

export function GrantDialog({
  issuer,
  close,
  created,
}: {
  issuer: string;
  close: () => void;
  created: () => void;
}) {
  const [mode, setMode] = useState('token');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<GrantResult>();
  const [error, setError] = useState('');
  async function submit(input: GrantInput) {
    setBusy(true);
    setError('');
    try {
      const grant = await createConnection(
        mode === 'oauth'
          ? input
          : { label: input.label, permissions: input.permissions, readScope: input.readScope },
      );
      setResult(grant);
      created();
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建连接失败');
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      open
      title={result ? '连接已创建' : '新建 AI 连接'}
      onCancel={() => {
        if (!busy) close();
      }}
      mask={{ closable: false }}
      cancelText="取消"
      confirmLoading={busy}
      okText="创建连接"
      okButtonProps={{ htmlType: 'submit', form: 'mcp-grant' }}
      footer={
        result ? (
          <Button onClick={close} type="primary">
            完成
          </Button>
        ) : undefined
      }
    >
      {result ? (
        <Space orientation="vertical" size={12} style={{ maxWidth: '100%' }}>
          <Typography.Text>在 AI 客户端选择 Streamable HTTP，填入接入地址。</Typography.Text>
          <Typography.Text copyable style={{ overflowWrap: 'anywhere' }}>
            {result.endpoint}
          </Typography.Text>
          {result.token ? (
            <>
              <Alert showIcon type="warning" title="令牌仅显示一次。请复制到客户端的 Bearer Token 字段。" />
              <Typography.Text code copyable style={{ overflowWrap: 'anywhere' }}>
                {result.token}
              </Typography.Text>
            </>
          ) : (
            <Typography.Text>在客户端选择 OAuth 并登录已绑定的账号。</Typography.Text>
          )}
        </Space>
      ) : (
        <Form<GrantInput>
          id="mcp-grant"
          layout="vertical"
          onFinish={submit}
          initialValues={{ permissions: defaultPermissions, readScope: 'own' }}
        >
          <Form.Item
            name="label"
            label="连接名称"
            rules={[{ required: true, whitespace: true, message: '请填写连接名称' }]}
          >
            <Input autoFocus placeholder="例如：博客写作助手" />
          </Form.Item>
          <Form.Item label="认证方式">
            <Select
              value={mode}
              onChange={setMode}
              options={[
                { value: 'token', label: 'Bearer 令牌' },
                ...(issuer ? [{ value: 'oauth', label: 'OAuth' }] : []),
              ]}
            />
          </Form.Item>
          {mode === 'oauth' && (
            <>
              <Form.Item name="oauthClient" label="客户端 ID" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
              <Form.Item name="oauthSubject" label="授权主体（sub）" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
            </>
          )}
          <Form.Item name="readScope" label="数据可见范围">
            <ReadScopePicker />
          </Form.Item>
          <Form.Item name="permissions" label="工具权限">
            <PermissionPicker />
          </Form.Item>
          {error && <Alert type="error" showIcon title={error} />}
        </Form>
      )}
    </Modal>
  );
}
