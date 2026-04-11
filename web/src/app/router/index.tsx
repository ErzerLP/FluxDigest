import { Navigate, Route, Routes } from 'react-router-dom';

import { AppLayout } from '../layout/AppLayout';

export type AppNavigationItem = {
  path: string;
  label: string;
  description: string;
};

export const appNavigation: AppNavigationItem[] = [
  { path: '/dashboard', label: 'Dashboard', description: '运行概览与系统健康' },
  { path: '/configs/llm', label: 'LLM Config', description: '模型入口与密钥策略' },
  { path: '/configs/miniflux', label: 'Miniflux', description: '订阅源同步与接入状态' },
  { path: '/configs/prompts', label: 'Prompts', description: '翻译分析提示词管理' },
  { path: '/configs/publish', label: 'Publish', description: '发布通道与输出策略' },
  { path: '/jobs', label: 'Jobs', description: '任务触发与运行记录' },
];

type PlaceholderPageProps = {
  eyebrow: string;
  title: string;
  description: string;
};

function PlaceholderPage({ eyebrow, title, description }: PlaceholderPageProps) {
  return (
    <section className="console-panel placeholder-page">
      <p className="section-eyebrow">{eyebrow}</p>
      <h2>{title}</h2>
      <p className="section-description">{description}</p>
      <div className="placeholder-grid">
        <article className="placeholder-block">
          <span className="placeholder-label">Scope</span>
          <strong>App shell ready</strong>
          <p>路由、主题、Provider 与 API client 已对齐当前后端能力。</p>
        </article>
        <article className="placeholder-block">
          <span className="placeholder-label">Next</span>
          <strong>Feature surface pending</strong>
          <p>后续任务可在此基础上接入 dashboard 卡片、配置表单与 jobs 列表。</p>
        </article>
      </div>
    </section>
  );
}

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<AppLayout navigation={appNavigation} />}>
        <Route index element={<Navigate replace to="/dashboard" />} />
        <Route
          path="dashboard"
          element={
            <PlaceholderPage
              eyebrow="Overview"
              title="System Overview"
              description="观察当前系统健康、集成状态与最近一次摘要运行结果。"
            />
          }
        />
        <Route
          path="configs/llm"
          element={
            <PlaceholderPage
              eyebrow="Configuration"
              title="Model Endpoint Controls"
              description="管理 base URL、模型与 API key 的最小可用配置入口。"
            />
          }
        />
        <Route
          path="configs/miniflux"
          element={
            <PlaceholderPage
              eyebrow="Configuration"
              title="Reader Intake"
              description="预留 Miniflux 接入检查与同步窗口配置页。"
            />
          }
        />
        <Route
          path="configs/prompts"
          element={
            <PlaceholderPage
              eyebrow="Configuration"
              title="Prompt Templates"
              description="预留翻译、分析提示词模板的版本管理与验证入口。"
            />
          }
        />
        <Route
          path="configs/publish"
          element={
            <PlaceholderPage
              eyebrow="Configuration"
              title="Publish Targets"
              description="预留发布目标、导出模式与渠道健康状态配置页。"
            />
          }
        />
        <Route
          path="jobs"
          element={
            <PlaceholderPage
              eyebrow="Operations"
              title="Recent Executions"
              description="查看 job runs 并为后续人工触发与调试流程预留位置。"
            />
          }
        />
        <Route path="*" element={<Navigate replace to="/dashboard" />} />
      </Route>
    </Routes>
  );
}
