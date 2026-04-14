import { useState } from 'react';
import { Alert, Button } from 'antd';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useAdminLogout } from '../../services/mutations/admin';
import { useAdminCurrentUser } from '../../services/queries/admin';
import type { AppNavigationItem } from '../router';

type AppLayoutProps = {
  navigation: AppNavigationItem[];
};

export function AppLayout({ navigation }: AppLayoutProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const currentUserQuery = useAdminCurrentUser();
  const logoutMutation = useAdminLogout();
  const [logoutErrorMessage, setLogoutErrorMessage] = useState<string>();
  const currentItem =
    navigation.find((item) => location.pathname === item.path) ?? navigation[0];
  const username = currentUserQuery.data?.username?.trim() || 'admin';

  async function handleLogout() {
    setLogoutErrorMessage(undefined);
    try {
      await logoutMutation.mutateAsync();
      navigate('/login', { replace: true });
    } catch (error) {
      setLogoutErrorMessage(error instanceof Error ? error.message : '退出登录失败，请稍后重试');
    }
  }

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <div className="brand-lockup">
          <span className="brand-kicker">Self-hosted AI digest</span>
          <h1>FluxDigest</h1>
          <p>为个人知识流准备的冷静控制台，聚焦运行态而非花哨装饰。</p>
        </div>

        <nav className="nav-stack" aria-label="Primary">
          {navigation.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) =>
                isActive ? 'nav-item nav-item-active' : 'nav-item'
              }
            >
              <span className="nav-item-label">{item.label}</span>
              <span className="nav-item-description">{item.description}</span>
            </NavLink>
          ))}
        </nav>

        <div className="sidebar-footnote">
          <span className="status-dot" />
          Shell scaffold online
        </div>
      </aside>

      <div className="app-main">
        <header className="topbar">
          <div>
            <p className="topbar-kicker">Control Surface</p>
            <h2>{currentItem.description}</h2>
          </div>
          <div className="topbar-meta">
            <span className="meta-chip meta-chip-subtle">Admin / {username}</span>
            {currentUserQuery.data?.must_change_password ? (
              <span className="meta-chip meta-chip-warning">Default password</span>
            ) : null}
            <span className="meta-chip">Graphite / Cyan</span>
            <span className="meta-chip meta-chip-subtle">MVP shell</span>
            <Button
              type="default"
              className="topbar-logout"
              loading={logoutMutation.isPending}
              onClick={() => {
                void handleLogout();
              }}
            >
              退出登录
            </Button>
          </div>
        </header>

        <main className="content-shell">
          {logoutErrorMessage ? (
            <Alert
              type="error"
              showIcon
              message="退出登录失败"
              description={logoutErrorMessage}
              style={{ marginBottom: '1rem' }}
            />
          ) : null}
          <Outlet />
        </main>
      </div>
    </div>
  );
}
