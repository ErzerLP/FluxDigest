import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { PublishConfigPage } from '../pages/configs/publish/PublishConfigPage';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderPage(ui: ReactElement) {
  return render(<AppProviders>{ui}</AppProviders>);
}

test('publish config page switches provider-specific fields and saves payload', async () => {
  const putSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          publish: {
            provider: 'halo',
            halo_base_url: 'http://127.0.0.1:8090',
            halo_token: { is_set: true, masked_value: 'halo****' },
            output_dir: '/srv/fluxdigest/output',
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/status') && method === 'GET') {
      return new Response(
        JSON.stringify({
          integrations: {
            publisher: {
              configured: true,
              last_test_status: 'ok',
            },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/configs/publish') && method === 'PUT') {
      putSpy(JSON.parse(String(init?.body ?? '')));
      return new Response(JSON.stringify({ ok: true }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<PublishConfigPage />);

  expect(await screen.findByLabelText('Provider')).toHaveValue('halo');
  expect(screen.getByLabelText('Halo Base URL')).toBeInTheDocument();

  await userEvent.selectOptions(screen.getByLabelText('Provider'), 'markdown_export');
  expect(screen.getByLabelText('Output Directory')).toBeInTheDocument();

  await userEvent.clear(screen.getByLabelText('Output Directory'));
  await userEvent.type(screen.getByLabelText('Output Directory'), '/data/digests');
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      provider: 'markdown_export',
      output_dir: '/data/digests',
      halo_token: { mode: 'keep' },
    }),
  );
});

test('publish config page can trigger connectivity test', async () => {
  const postSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          publish: {
            provider: 'halo',
            halo_base_url: 'http://127.0.0.1:8090',
            halo_token: { is_set: true, masked_value: 'halo****' },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/status') && method === 'GET') {
      return new Response(
        JSON.stringify({
          integrations: {
            publisher: {
              configured: true,
              last_test_status: 'configured',
            },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/test/publish') && method === 'POST') {
      postSpy();
      return new Response(JSON.stringify({ status: 'ok', message: 'publish connected' }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<PublishConfigPage />);

  await userEvent.click(await screen.findByRole('button', { name: '测试连接' }));

  expect(postSpy).toHaveBeenCalledTimes(1);
  expect(await screen.findByText('publish connected')).toBeInTheDocument();
});
