import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { App, Button, Checkbox, Form, Input, Typography } from 'antd';
import { AppstoreOutlined, LockOutlined, MailOutlined, SafetyOutlined } from '@ant-design/icons';
import {
  isLoginMFAChallenge,
  loginAdmin,
  resendLoginCode,
  setAuthToken,
  setAuthUser,
  setRefreshToken,
  verifyLoginCode,
} from '../../api/auth';
import type { LoginRequest, LoginResponse } from '../../api/auth';
import './LoginPage.css';

const { Title, Text } = Typography;

const RESEND_COOLDOWN_SECONDS = 60;

interface LoginLocationState {
  from?: {
    pathname: string;
    search: string;
    hash: string;
  };
}

interface MFAChallenge {
  email: string;
  challengeToken: string;
}

function LoginForm() {
  const navigate = useNavigate();
  const location = useLocation();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [stage, setStage] = useState<'credentials' | 'code'>('credentials');
  const [challenge, setChallenge] = useState<MFAChallenge | null>(null);
  const [code, setCode] = useState('');
  const [codeLoading, setCodeLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const [resendCooldown, setResendCooldown] = useState(0);
  const from = (location.state as LoginLocationState | null)?.from;
  const redirectParam = new URLSearchParams(location.search).get('redirect');
  const redirectTo = from
    ? `${from.pathname}${from.search}${from.hash}`
    : normalizeAdminRedirect(redirectParam);

  useEffect(() => {
    if (resendCooldown <= 0) return;
    const timer = setTimeout(() => setResendCooldown(resendCooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [resendCooldown]);

  const applyLogin = (response: LoginResponse) => {
    setAuthToken(response.token);
    setRefreshToken(response.refreshToken);
    setAuthUser(response.user);
    void message.success('登录成功');
    navigate(redirectTo, { replace: true });
  };

  const handleSubmit = async (values: LoginRequest) => {
    setLoading(true);
    try {
      const response = await loginAdmin(values);
      if (isLoginMFAChallenge(response)) {
        setChallenge({ email: response.email, challengeToken: response.challengeToken });
        setCode('');
        setResendCooldown(RESEND_COOLDOWN_SECONDS);
        setStage('code');
        void message.success(`验证码已发送至 ${response.email}，请查收`);
        return;
      }
      applyLogin(response);
    } catch (err) {
      const error = err as { message?: string };
      void message.error(error.message || '登录失败，请检查邮箱和密码');
    } finally {
      setLoading(false);
    }
  };

  const handleVerify = async () => {
    if (!challenge || code.length !== 6) return;
    setCodeLoading(true);
    try {
      const response = await verifyLoginCode({ challengeToken: challenge.challengeToken, code });
      applyLogin(response);
    } catch (err) {
      const error = err as { message?: string };
      void message.error(error.message || '验证码错误或已过期');
      if (error.message === '登录验证已失效，请重新登录') {
        backToCredentials();
      }
    } finally {
      setCodeLoading(false);
    }
  };

  const handleResend = async () => {
    if (!challenge || resendCooldown > 0 || resending) return;
    setResending(true);
    try {
      await resendLoginCode({ challengeToken: challenge.challengeToken });
      setResendCooldown(RESEND_COOLDOWN_SECONDS);
      void message.success('验证码已重新发送，请查收');
    } catch (err) {
      const error = err as { message?: string };
      void message.error(error.message || '验证码发送失败，请稍后再试');
      if (error.message === '登录验证已失效，请重新登录') {
        backToCredentials();
      }
    } finally {
      setResending(false);
    }
  };

  const backToCredentials = () => {
    setStage('credentials');
    setChallenge(null);
    setCode('');
    setResendCooldown(0);
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
          {stage === 'credentials' ? (
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
          ) : (
            <div className="login-mfa">
              <div className="login-mfa-icon" aria-hidden="true">
                <SafetyOutlined />
              </div>
              <Text className="login-mfa-hint">
                检测到登录环境变化，验证码已发送至 <Text strong>{challenge?.email}</Text>
                ，请输入 6 位数字验证码完成登录。
              </Text>
              <Input.OTP
                length={6}
                autoFocus
                value={code}
                onChange={(value) => setCode(value.replace(/\D/g, ''))}
              />
              <Button
                type="primary"
                className="login-button login-mfa-submit"
                loading={codeLoading}
                disabled={code.length !== 6}
                onClick={handleVerify}
                block
              >
                验证并登录
              </Button>
              <div className="login-mfa-links">
                <Button
                  type="link"
                  size="small"
                  className="login-mfa-link"
                  disabled={resendCooldown > 0 || resending}
                  onClick={handleResend}
                >
                  {resendCooldown > 0 ? `重新发送验证码（${resendCooldown}s）` : '重新发送验证码'}
                </Button>
                <Button
                  type="link"
                  size="small"
                  className="login-mfa-link"
                  onClick={backToCredentials}
                >
                  返回重新登录
                </Button>
              </div>
            </div>
          )}
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
