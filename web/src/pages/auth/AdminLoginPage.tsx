import { Alert, Button, Form, Input } from 'antd';
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom';

import { isAdminUnauthorizedError } from '../../services/api/admin';
import { useAdminLogin } from '../../services/mutations/admin';
import { useAdminCurrentUser } from '../../services/queries/admin';

type LoginFormValues = {
  username: string;
  password: string;
};

const defaultAdminUsername = 'FluxDigest';
const defaultAdminPassword = 'FluxDigest';

function resolveRedirectTarget(rawRedirect: string | null) {
  if (!rawRedirect || !rawRedirect.startsWith('/') || rawRedirect.startsWith('//')) {
    return '/dashboard';
  }
  if (rawRedirect === '/login') {
    return '/dashboard';
  }
  return rawRedirect;
}

export function AdminLoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const currentUserQuery = useAdminCurrentUser();
  const loginMutation = useAdminLogin();
  const redirectTarget = resolveRedirectTarget(searchParams.get('redirect'));

  if (currentUserQuery.data?.username) {
    return <Navigate replace to={redirectTarget} />;
  }

  async function handleSubmit(values: LoginFormValues) {
    try {
      await loginMutation.mutateAsync(values);
      navigate(redirectTarget, { replace: true });
    } catch {
      // 错误态由 mutation.error 驱动页面提示，这里吞掉 Promise 以避免未处理拒绝。
    }
  }

  const isUnexpectedSessionError =
    currentUserQuery.isError && !isAdminUnauthorizedError(currentUserQuery.error);
  const loginErrorMessage =
    loginMutation.error instanceof Error ? loginMutation.error.message : undefined;

  return (
    <div className="auth-screen">
      <section className="auth-card">
        <div className="brand-lockup auth-brand">
          <span className="brand-kicker">Admin Access</span>
          <h1>Admin Login</h1>
          <p>登录后才能访问 FluxDigest 控制台配置与任务面板。</p>
        </div>

        <Alert
          type="warning"
          showIcon
          message="首次安装默认账户/密码均为 FluxDigest。"
          description="如果这是你的首次部署，请先用默认账户进入控制台；后续建议尽快完成密码轮换。"
        />

        {isUnexpectedSessionError ? (
          <Alert
            type="error"
            showIcon
            message="无法确认当前登录状态"
            description={
              currentUserQuery.error instanceof Error
                ? currentUserQuery.error.message
                : '管理员会话探测失败'
            }
          />
        ) : null}

        {loginErrorMessage ? (
          <Alert type="error" showIcon message="登录失败" description={loginErrorMessage} />
        ) : null}

        <Form<LoginFormValues>
          layout="vertical"
          initialValues={{
            username: defaultAdminUsername,
            password: defaultAdminPassword,
          }}
          onFinish={(values) => {
            void handleSubmit(values);
          }}
          className="auth-form"
        >
          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: '请输入管理员用户名' }]}
          >
            <Input autoComplete="username" placeholder="FluxDigest" />
          </Form.Item>

          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: '请输入管理员密码' }]}
          >
            <Input.Password autoComplete="current-password" placeholder="FluxDigest" />
          </Form.Item>

          <Button
            type="primary"
            htmlType="submit"
            size="large"
            block
            loading={loginMutation.isPending}
            disabled={currentUserQuery.isLoading}
          >
            登录控制台
          </Button>
        </Form>

        <div className="auth-footnote">
          Session Cookie：<span className="monospace">fluxdigest_admin_session</span>
        </div>
      </section>
    </div>
  );
}
