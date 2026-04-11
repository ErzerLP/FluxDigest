import { NavLink, Outlet, useLocation } from 'react-router-dom';

import type { AppNavigationItem } from '../router';

type AppLayoutProps = {
  navigation: AppNavigationItem[];
};

export function AppLayout({ navigation }: AppLayoutProps) {
  const location = useLocation();
  const currentItem =
    navigation.find((item) => location.pathname === item.path) ?? navigation[0];

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
            <span className="meta-chip">Graphite / Cyan</span>
            <span className="meta-chip meta-chip-subtle">MVP shell</span>
          </div>
        </header>

        <main className="content-shell">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
