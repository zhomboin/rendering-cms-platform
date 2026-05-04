import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Card, Form, Input, Button, Typography, App } from 'antd';
import { MailOutlined, LockOutlined } from '@ant-design/icons';
import { loginAdmin, setAuthToken, setAuthUser } from '../../api/auth';
import type { LoginRequest } from '../../api/auth';

const { Title } = Typography;

interface LoginLocationState {
  from?: {
    pathname: string;
    search: string;
    hash: string;
  };
}

function LoginForm() {
  const navigate = useNavigate();
  const location = useLocation();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const from = (location.state as LoginLocationState | null)?.from;
  const redirectParam = new URLSearchParams(location.search).get('redirect');
  const redirectTo = from
    ? `${from.pathname}${from.search}${from.hash}`
    : normalizeAdminRedirect(redirectParam);

  const handleSubmit = async (values: LoginRequest) => {
    setLoading(true);
    try {
      const response = await loginAdmin(values);
      setAuthToken(response.token);
      setAuthUser(response.user);
      void message.success('登录成功');
      navigate(redirectTo, { replace: true });
    } catch (err) {
      const error = err as { message?: string };
      void message.error(error.message || '登录失败，请检查邮箱和密码');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        background: '#F8FAFC',
        padding: 24,
      }}
    >
      <Card
        style={{
          width: '100%',
          maxWidth: 400,
          borderRadius: 24,
          boxShadow: '0 4px 24px rgba(0, 0, 0, 0.06)',
        }}
        styles={{ body: { padding: 48 } }}
      >
        <Title
          level={3}
          style={{
            textAlign: 'center',
            fontSize: 24,
            fontWeight: 700,
            marginBottom: 32,
            color: '#0F172A',
          }}
        >
          后台登录
        </Title>

        <Form<LoginRequest>
          layout="vertical"
          onFinish={handleSubmit}
          autoComplete="off"
          size="large"
        >
          <Form.Item
            name="email"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '请输入有效的邮箱地址' },
            ]}
          >
            <Input
              prefix={<MailOutlined />}
              placeholder="管理员邮箱"
              style={{ height: 44, borderRadius: 8 }}
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="密码"
              style={{ height: 44, borderRadius: 8 }}
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 16 }}>
            <Button
              type="primary"
              htmlType="submit"
              loading={loading}
              block
              style={{
                height: 44,
                borderRadius: 8,
                fontSize: 16,
                fontWeight: 600,
              }}
            >
              登录
            </Button>
          </Form.Item>
        </Form>

        <p
          style={{
            textAlign: 'center',
            fontSize: 12,
            color: '#64748B',
            margin: 0,
          }}
        >
          仅限管理员登录
        </p>
      </Card>
    </div>
  );
}

function normalizeAdminRedirect(value: string | null) {
  if (!value) return '/admin';
  if (!value.startsWith('/admin') || value.startsWith('/admin/login')) return '/admin';
  return value;
}

export default function LoginPage() {
  return (
    <App>
      <LoginForm />
    </App>
  );
}
