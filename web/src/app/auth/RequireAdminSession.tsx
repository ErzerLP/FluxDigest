import { Alert, Spin } from 'antd';
import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { isAdminUnauthorizedError } from '../../services/api/admin';
import { useAdminCurrentUser } from '../../services/queries/admin';

function resolveRedirectTarget(pathname: string, search: string, hash: string) {
  const target = `${pathname}${search}${hash}`;
  if (!target || target === '/login') {
    return '/dashboard';
  }
  return target;
}

export function RequireAdminSession() {
  const location = useLocation();
  const currentUserQuery = useAdminCurrentUser();

  if (currentUserQuery.isLoading) {
    return (
      <div className="auth-screen">
        <Spin size="large" />
        <p>正在验证管理员会话…</p>
      </div>
    );
  }

  if (currentUserQuery.isError) {
    if (isAdminUnauthorizedError(currentUserQuery.error)) {
      const redirect = resolveRedirectTarget(location.pathname, location.search, location.hash);
      return <Navigate replace to={`/login?redirect=${encodeURIComponent(redirect)}`} />;
    }

    return (
      <div className="auth-screen">
        <div className="auth-card auth-card-compact">
          <Alert
            type="error"
            showIcon
            message="管理员会话校验失败"
            description={
              currentUserQuery.error instanceof Error
                ? currentUserQuery.error.message
                : '无法确认当前管理员会话状态'
            }
          />
        </div>
      </div>
    );
  }

  if (!currentUserQuery.data?.username) {
    return <Navigate replace to="/login" />;
  }

  return <Outlet />;
}
