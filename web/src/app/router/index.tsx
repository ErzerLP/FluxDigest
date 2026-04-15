import { Navigate, Route, Routes } from 'react-router-dom';

import { RequireAdminSession } from '../auth/RequireAdminSession';
import { LLMConfigPage } from '../../pages/configs/llm/LLMConfigPage';
import { MinifluxConfigPage } from '../../pages/configs/miniflux/MinifluxConfigPage';
import { PromptConfigPage } from '../../pages/configs/prompts/PromptConfigPage';
import { PublishConfigPage } from '../../pages/configs/publish/PublishConfigPage';
import { AdminLoginPage } from '../../pages/auth/AdminLoginPage';
import { DashboardPage } from '../../pages/dashboard/DashboardPage';
import { JobsPage } from '../../pages/jobs/JobsPage';
import { AppLayout } from '../layout/AppLayout';

export type AppNavigationItem = {
  path: string;
  label: string;
  description: string;
};

export const appNavigation: AppNavigationItem[] = [
  { path: '/dashboard', label: '总览', description: '运行概览与系统健康' },
  { path: '/configs/llm', label: '模型配置', description: '模型入口与密钥策略' },
  { path: '/configs/miniflux', label: '订阅源', description: '订阅源同步与接入状态' },
  { path: '/configs/prompts', label: '提示词', description: '翻译分析提示词管理' },
  { path: '/configs/publish', label: '发布设置', description: '发布通道与输出策略' },
  { path: '/jobs', label: '任务记录', description: '任务触发与运行记录' },
];

export function AppRouter() {
  return (
    <Routes>
      <Route path="/login" element={<AdminLoginPage />} />
      <Route element={<RequireAdminSession />}>
        <Route path="/" element={<AppLayout navigation={appNavigation} />}>
          <Route index element={<Navigate replace to="/dashboard" />} />
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="configs/llm" element={<LLMConfigPage />} />
          <Route path="configs/miniflux" element={<MinifluxConfigPage />} />
          <Route path="configs/prompts" element={<PromptConfigPage />} />
          <Route path="configs/publish" element={<PublishConfigPage />} />
          <Route path="jobs" element={<JobsPage />} />
          <Route path="*" element={<Navigate replace to="/dashboard" />} />
        </Route>
      </Route>
    </Routes>
  );
}
