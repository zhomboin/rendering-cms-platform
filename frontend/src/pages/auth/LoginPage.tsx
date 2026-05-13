import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { App, Button, Checkbox, Form, Input, Typography } from 'antd';
import { AppstoreOutlined, LockOutlined, MailOutlined } from '@ant-design/icons';
import { loginAdmin, setAuthToken, setAuthUser } from '../../api/auth';
import type { LoginRequest } from '../../api/auth';
import './LoginPage.css';

const { Title, Text } = Typography;

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
    <main className="login-page">
      <section className="login-shell" aria-label="Rendering CMS 登录">
        <div className="login-heading">
          <div className="login-logo" aria-hidden="true">
            <AppstoreOutlined />
          </div>
          <Title level={1}>Rendering CMS</Title>
          <Text>欢迎回来，请使用管理员账号登录。</Text>
        </div>

        <div className="login-card">
          <Form<LoginRequest>
            className="login-form"
            layout="vertical"
            onFinish={handleSubmit}
            autoComplete="off"
            size="large"
          >
            <Form.Item
              label="工作邮箱"
              name="email"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '请输入有效的邮箱地址' },
              ]}
            >
              <Input
                prefix={<MailOutlined />}
                className="login-input"
                autoComplete="email"
              />
            </Form.Item>

            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, message: '请输入密码' }]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                className="login-input"
                autoComplete="current-password"
              />
            </Form.Item>

            <div className="login-options">
              <Checkbox className="login-remember">保持登录状态</Checkbox>
              <a className="login-help" href="mailto:admin@rendering.local">
                忘记密码?
              </a>
            </div>

            <Form.Item className="login-action">
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                className="login-button"
                block
              >
                登录
              </Button>
            </Form.Item>
          </Form>
        </div>

        <p className="login-footnote">
          &copy; 2026 Rendering CMS. 仅限授权管理员访问。
        </p>
      </section>
    </main>
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
