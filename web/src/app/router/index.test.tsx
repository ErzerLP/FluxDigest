import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, vi } from 'vitest';

import { AppProviders } from '../providers/AppProviders';
import { AppRouter } from './index';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderRouter(initialEntries: string[]) {
  render(
    <AppProviders>
      <MemoryRouter initialEntries={initialEntries}>
        <AppRouter />
      </MemoryRouter>
    </AppProviders>,
  );
}

function jsonResponse(payload: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function mockAuthenticatedConsole() {
  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/auth/me') && method === 'GET') {
      return jsonResponse({
        user_id: 'admin-1',
        username: 'FluxDigest',
        must_change_password: true,
      });
    }

    if (url.endsWith('/api/v1/admin/status') && method === 'GET') {
      return jsonResponse({
        system: { api: 'ok', db: 'ok', redis: 'ok' },
        integrations: { llm: { configured: true, last_test_status: 'ok' } },
        runtime: { latest_digest_date: '2026-04-14', latest_digest_status: 'ok' },
      });
    }

    if (url.includes('/api/v1/admin/jobs') && method === 'GET') {
      return jsonResponse({ items: [] });
    }

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return jsonResponse({
        publish: { provider: 'halo', article_publish_mode: 'digest_only', article_review_mode: 'manual_review' },
        scheduler: { enabled: true, schedule_time: '07:00', timezone: 'Asia/Shanghai' },
      });
    }

    return new Response('not found', { status: 404 });
  });
}

test('renders dashboard navigation item and dashboard placeholder content', async () => {
  mockAuthenticatedConsole();
  renderRouter(['/dashboard']);

  expect(await screen.findByText('总览')).toBeInTheDocument();
  expect(screen.getByText('FluxDigest')).toBeInTheDocument();
  expect(
    screen.getByText('观察当前系统健康、集成状态与最近一次摘要运行结果。'),
  ).toBeInTheDocument();
});

test('falls back unknown routes to dashboard content', async () => {
  mockAuthenticatedConsole();
  renderRouter(['/unknown']);

  expect(
    await screen.findByText('观察当前系统健康、集成状态与最近一次摘要运行结果。'),
  ).toBeInTheDocument();
  expect(screen.getByText('总览')).toBeInTheDocument();
});

test('redirects index route to dashboard content', async () => {
  mockAuthenticatedConsole();
  renderRouter(['/']);

  expect(
    await screen.findByText('观察当前系统健康、集成状态与最近一次摘要运行结果。'),
  ).toBeInTheDocument();
});
