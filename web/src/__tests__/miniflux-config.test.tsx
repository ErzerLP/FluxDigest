import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { MinifluxConfigPage } from '../pages/configs/miniflux/MinifluxConfigPage';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderPage(ui: ReactElement) {
  return render(<AppProviders>{ui}</AppProviders>);
}

test('miniflux config page loads snapshot and saves keep-token payload', async () => {
  const putSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          miniflux: {
            base_url: 'http://127.0.0.1:28082',
            fetch_limit: 120,
            lookback_hours: 48,
            api_token: { is_set: true, masked_value: 'mini****' },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/status') && method === 'GET') {
      return new Response(
        JSON.stringify({
          integrations: {
            miniflux: {
              configured: true,
              last_test_status: 'ok',
              last_test_at: '2026-04-14T07:00:00Z',
            },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/configs/miniflux') && method === 'PUT') {
      putSpy(JSON.parse(String(init?.body ?? '')));
      return new Response(JSON.stringify({ ok: true }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<MinifluxConfigPage />);

  expect(await screen.findByLabelText('Base URL')).toHaveValue('http://127.0.0.1:28082');
  expect(screen.getByLabelText('Fetch Limit')).toHaveValue(120);
  expect(screen.getByLabelText('Lookback Hours')).toHaveValue(48);

  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      base_url: 'http://127.0.0.1:28082',
      fetch_limit: 120,
      lookback_hours: 48,
      api_token: { mode: 'keep' },
    }),
  );
});

test('miniflux config page can trigger connectivity test with saved config', async () => {
  const postSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          miniflux: {
            base_url: 'http://127.0.0.1:28082',
            fetch_limit: 100,
            lookback_hours: 24,
            api_token: { is_set: true, masked_value: 'mini****' },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/status') && method === 'GET') {
      return new Response(
        JSON.stringify({
          integrations: {
            miniflux: {
              configured: true,
              last_test_status: 'configured',
            },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/test/miniflux') && method === 'POST') {
      postSpy();
      return new Response(JSON.stringify({ status: 'ok', message: 'miniflux connected' }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<MinifluxConfigPage />);

  await userEvent.click(await screen.findByRole('button', { name: '测试连接' }));

  expect(postSpy).toHaveBeenCalledTimes(1);
  expect(await screen.findByText('miniflux connected')).toBeInTheDocument();
});
