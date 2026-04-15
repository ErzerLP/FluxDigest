import '@testing-library/jest-dom/vitest';
import { render, screen, waitFor } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { AppRouter } from '../app/router';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

function renderRouter(initialEntries: string[]) {
  return render(
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

test('redirects unauthenticated admin routes to login page', async () => {
  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/auth/me') && method === 'GET') {
      return jsonResponse({ error: 'admin session required' }, { status: 401 });
    }

    return new Response('not found', { status: 404 });
  });

  renderRouter(['/configs/miniflux']);

  expect(await screen.findByRole('heading', { name: 'Admin Login' })).toBeInTheDocument();
  expect(screen.getByText('登录后才能访问 FluxDigest 控制台配置与任务面板。')).toBeInTheDocument();
});

test('login page can authenticate and navigate back to protected route', async () => {
  const loginPayloads: Array<Record<string, unknown>> = [];
  let loggedIn = false;

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/auth/me') && method === 'GET') {
      if (!loggedIn) {
        return jsonResponse({ error: 'admin session required' }, { status: 401 });
      }
      return jsonResponse({
        user_id: 'admin-1',
        username: 'FluxDigest',
        must_change_password: true,
      });
    }

    if (url.endsWith('/api/v1/admin/auth/login') && method === 'POST') {
      loginPayloads.push(JSON.parse(String(init?.body ?? '{}')));
      loggedIn = true;
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

    return new Response('not found', { status: 404 });
  });

  renderRouter(['/configs/prompts']);

  await screen.findByLabelText('用户名');
  await userEvent.click(screen.getByRole('button', { name: '登录控制台' }));

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: '翻译分析提示词管理' })).toBeInTheDocument();
  });

  expect(loginPayloads).toEqual([
    {
      username: 'FluxDigest',
      password: 'FluxDigest',
    },
  ]);
  expect(screen.getByText('FluxDigest')).toBeInTheDocument();
});

test('login page keeps user on login view when credentials are invalid', async () => {
  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/auth/me') && method === 'GET') {
      return jsonResponse({ error: 'admin session required' }, { status: 401 });
    }

    if (url.endsWith('/api/v1/admin/auth/login') && method === 'POST') {
      return jsonResponse({ error: 'invalid admin credentials' }, { status: 401 });
    }

    return new Response('not found', { status: 404 });
  });

  renderRouter(['/configs/publish']);

  await screen.findByRole('heading', { name: 'Admin Login' });
  await userEvent.click(screen.getByRole('button', { name: '登录控制台' }));

  expect(await screen.findByText('登录失败')).toBeInTheDocument();
  expect(screen.getByText('invalid admin credentials')).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: 'Admin Login' })).toBeInTheDocument();
});

test('logout failure stays on protected route and shows feedback', async () => {
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

    if (url.endsWith('/api/v1/admin/auth/logout') && method === 'POST') {
      return jsonResponse({ error: 'logout unavailable' }, { status: 503 });
    }

    return new Response('not found', { status: 404 });
  });

  renderRouter(['/dashboard']);

  await screen.findByRole('heading', { name: '运行概览与系统健康' });
  await userEvent.click(screen.getByRole('button', { name: '退出登录' }));

  expect(await screen.findByText('logout unavailable')).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: '运行概览与系统健康' })).toBeInTheDocument();
});
