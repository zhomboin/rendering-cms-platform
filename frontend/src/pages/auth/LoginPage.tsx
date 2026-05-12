import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Form, Input, Button, Typography, App } from 'antd';
import { MailOutlined, LockOutlined } from '@ant-design/icons';
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
      <section className="login-card" aria-label="Rendering CMS 登录">
        <div className="login-visual" aria-hidden="true">
          <div className="login-visual__overlay" />
          <div className="login-visual__caption">
            <span>Rendering CMS</span>
            <strong>内容管理平台</strong>
          </div>
        </div>

        <div className="login-panel">
          <div className="login-panel__inner">
            <div className="login-heading">
              <Title level={2}>Welcome</Title>
              <Text>登录账号后继续管理文章、评论与资源</Text>
            </div>

            <Form<LoginRequest>
              className="login-form"
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
                  placeholder="admin@rendering.local"
                  className="login-input"
                />
              </Form.Item>

              <Form.Item
                name="password"
                rules={[{ required: true, message: '请输入密码' }]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder="请输入密码"
                  className="login-input"
                />
              </Form.Item>

              <a className="login-help" href="mailto:admin@rendering.local">
                忘记密码请联系管理员
              </a>

              <Form.Item className="login-action">
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={loading}
                  className="login-button"
                >
                  登录
                </Button>
              </Form.Item>
            </Form>

            <p className="login-footnote">仅限授权管理员登录</p>
          </div>
        </div>
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
