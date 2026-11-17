import { useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { App, Button, Form, Input, Typography } from 'antd';
import { LockOutlined, MailOutlined, SafetyOutlined } from '@ant-design/icons';
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
  const verifying = useRef(false);
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

  const handleVerify = async (value: string) => {
    if (!challenge || !/^\d{6}$/.test(value) || verifying.current || resending) return;
    verifying.current = true;
    setCodeLoading(true);
    try {
      const response = await verifyLoginCode({
        challengeToken: challenge.challengeToken,
        code: value,
      });
      applyLogin(response);
    } catch (err) {
      const error = err as { message?: string };
      void message.error(error.message || '验证码错误或已过期');
      if (error.message === '登录验证已失效，请重新登录') {
        backToCredentials();
      }
    } finally {
      verifying.current = false;
      setCodeLoading(false);
    }
  };

  const handleResend = async () => {
    if (!challenge || resendCooldown > 0 || resending || verifying.current) return;
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
            <img src="/logo/rendering-woodcut-128.png" width={48} height={48} alt="" />
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
                <Input prefix={<MailOutlined />} className="login-input" autoComplete="email" />
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
              <Text type="secondary" className="login-help">
                忘记密码？请联系系统管理员。
              </Text>
            </Form>
          ) : (
            <div className="login-mfa">
              <div className="login-mfa-icon" aria-hidden="true">
                <SafetyOutlined />
              </div>
              <Text className="login-mfa-hint">
                验证码已发送至 <Text strong>{challenge?.email}</Text>，请输入 6 位数字验证码。
              </Text>
              <Input.OTP
                role="group"
                aria-label="邮箱验证码"
                length={6}
                autoFocus
                autoComplete="one-time-code"
                disabled={codeLoading || resending}
                formatter={(value) => value.replace(/\D/g, '')}
                onInput={(cells) => setCode(cells.join(''))}
                onChange={(value) => void handleVerify(value)}
              />
              <Button
                type="primary"
                className="login-button login-mfa-submit"
                loading={codeLoading}
                disabled={code.length !== 6 || resending}
                onClick={() => void handleVerify(code)}
                block
              >
                验证并登录
              </Button>
              <div className="login-mfa-links">
                <Button
                  type="link"
                  size="small"
                  className="login-mfa-link"
                  disabled={resendCooldown > 0 || resending || codeLoading}
                  onClick={handleResend}
                >
                  {resendCooldown > 0 ? `重新发送验证码（${resendCooldown}s）` : '重新发送验证码'}
                </Button>
                <Button
                  type="link"
                  size="small"
                  className="login-mfa-link"
                  disabled={codeLoading || resending}
                  onClick={backToCredentials}
                >
                  返回重新登录
                </Button>
              </div>
            </div>
          )}
        </div>

        <p className="login-footnote">&copy; 2026 Rendering CMS. 仅限授权管理员访问。</p>
      </section>
    </main>
  );
}

function normalizeAdminRedirect(value: string | null) {
  if (!value) return '/admin';
  if (!value.startsWith('/admin') || value.startsWith('/admin/login')) return '/admin';
  return value;
}

export default LoginForm;
