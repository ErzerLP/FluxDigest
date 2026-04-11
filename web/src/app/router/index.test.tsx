import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

import { AppProviders } from '../providers/AppProviders';
import { AppRouter } from './index';

function renderRouter(initialEntries: string[]) {
  render(
    <AppProviders>
      <MemoryRouter initialEntries={initialEntries}>
        <AppRouter />
      </MemoryRouter>
    </AppProviders>,
  );
}

test('renders dashboard navigation item and dashboard placeholder content', async () => {
  renderRouter(['/dashboard']);

  expect(await screen.findByText('Dashboard')).toBeInTheDocument();
  expect(screen.getByText('FluxDigest')).toBeInTheDocument();
  expect(
    screen.getByText('观察当前系统健康、集成状态与最近一次摘要运行结果。'),
  ).toBeInTheDocument();
});

test('falls back unknown routes to dashboard content', async () => {
  renderRouter(['/unknown']);

  expect(
    await screen.findByText('观察当前系统健康、集成状态与最近一次摘要运行结果。'),
  ).toBeInTheDocument();
  expect(screen.getByText('Dashboard')).toBeInTheDocument();
});

test('redirects index route to dashboard content', async () => {
  renderRouter(['/']);

  expect(
    await screen.findByText('观察当前系统健康、集成状态与最近一次摘要运行结果。'),
  ).toBeInTheDocument();
});
