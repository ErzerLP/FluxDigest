import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { DashboardPage } from '../pages/dashboard/DashboardPage';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderPage(ui: ReactElement) {
  return render(<AppProviders>{ui}</AppProviders>);
}

test('dashboard renders latest digest and quick actions', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.endsWith('/api/v1/admin/status')) {
      return new Response(
        JSON.stringify({
          integrations: { llm: { configured: true, last_test_status: 'ok' } },
          runtime: { latest_digest_date: '2026-04-11', latest_job_status: 'succeeded' },
          system: { api: 'ok', db: 'ok', redis: 'ok' },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.includes('/api/v1/admin/jobs')) {
      return new Response(JSON.stringify({ items: [] }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<DashboardPage />);

  expect(await screen.findByText('2026-04-11')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '手动触发日报' })).toBeDisabled();
  expect(screen.getByText('Admin trigger 未接入，当前仅保留占位入口。')).toBeInTheDocument();
});

test('dashboard prefers latest digest status and falls back to latest job status', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.endsWith('/api/v1/admin/status')) {
      return new Response(
        JSON.stringify({
          runtime: {
            latest_digest_date: '2026-04-12',
            latest_digest_status: 'failed',
            latest_job_status: 'succeeded',
          },
          system: { api: 'ok', db: 'ok', redis: 'ok' },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.includes('/api/v1/admin/jobs')) {
      return new Response(JSON.stringify({ items: [] }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<DashboardPage />);

  expect(await screen.findByText('2026-04-12')).toBeInTheDocument();
  expect(screen.getByText('失败')).toBeInTheDocument();
});

test('dashboard shows recent jobs error state instead of empty state', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.endsWith('/api/v1/admin/status')) {
      return new Response(
        JSON.stringify({
          runtime: { latest_digest_date: '2026-04-12', latest_digest_status: 'succeeded' },
          system: { api: 'ok', db: 'ok', redis: 'ok' },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.includes('/api/v1/admin/jobs')) {
      return new Response(JSON.stringify({ error: 'jobs service unavailable' }), {
        status: 503,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<DashboardPage />);

  expect(await screen.findByText('最近任务读取失败')).toBeInTheDocument();
  expect(screen.queryByText('当前没有任务记录。')).not.toBeInTheDocument();
});
