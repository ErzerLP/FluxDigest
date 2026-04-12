import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { LLMConfigPage } from '../pages/configs/llm/LLMConfigPage';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderPage(ui: ReactElement) {
  return render(<AppProviders>{ui}</AppProviders>);
}

test('llm config page saves keep-secret payload', async () => {
  const putSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method = init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          llm: {
            base_url: 'https://llm.local/v1',
            model: 'gpt-4.1-mini',
            api_key: { is_set: true, masked_value: 'secr****' },
            is_enabled: true,
            timeout_ms: 30000,
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/configs/llm') && method === 'PUT') {
      const bodyText = typeof init?.body === 'string' ? init.body : '';
      putSpy(JSON.parse(bodyText));
      return new Response(JSON.stringify({ ok: true }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  await userEvent.clear(await screen.findByLabelText('Base URL'));
  await userEvent.type(screen.getByLabelText('Base URL'), 'https://proxy.local/v1');
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      base_url: expect.stringContaining('https://proxy.local/v1'),
      model: 'gpt-4.1-mini',
      api_key: { mode: 'keep' },
    }),
  );
  expect(putSpy).toHaveBeenCalledWith(
    expect.not.objectContaining({
      is_enabled: expect.anything(),
      timeout_ms: expect.anything(),
    }),
  );
  expect(screen.queryByLabelText('Timeout (ms)')).not.toBeInTheDocument();
  expect(screen.queryByLabelText('启用 LLM')).not.toBeInTheDocument();
});

test('llm config page blocks connection test in keep-secret mode', async () => {
  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method = init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          llm: {
            base_url: 'https://llm.local/v1',
            model: 'gpt-4.1-mini',
            api_key: { is_set: true, masked_value: 'secr****' },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/test/llm')) {
      throw new Error('keep mode should not send test request');
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  await userEvent.click(await screen.findByRole('button', { name: '测试连接' }));

  expect(
    await screen.findByText('测试连接需要切换为替换密钥并输入待测 key。'),
  ).toBeInTheDocument();
});

test('llm config page disables actions before config snapshot is ready', async () => {
  let resolveConfig: ((value: Response) => void) | undefined;

  fetchMock.mockImplementation(
    (async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method =
        init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

      if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
        return await new Promise<Response>((resolve) => {
          resolveConfig = resolve;
        });
      }

      if (url.endsWith('/api/v1/admin/configs/llm') || url.endsWith('/api/v1/admin/test/llm')) {
        throw new Error('actions should stay disabled before snapshot resolves');
      }

      return new Response('not found', { status: 404 });
    }) as typeof fetch,
  );

  renderPage(<LLMConfigPage />);

  expect(await screen.findByRole('button', { name: '测试连接' })).toBeDisabled();
  expect(screen.getByRole('button', { name: '保存配置' })).toBeDisabled();

  resolveConfig?.(
    new Response(
      JSON.stringify({
        llm: {
          base_url: 'https://llm.local/v1',
          model: 'gpt-4.1-mini',
          api_key: { is_set: true, masked_value: 'secr****' },
        },
      }),
      { headers: { 'Content-Type': 'application/json' } },
    ),
  );

  expect(await screen.findByLabelText('Base URL')).toBeEnabled();
});

test('llm config page keeps actions disabled when config snapshot fails', async () => {
  const putSpy = vi.fn();
  const postSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(JSON.stringify({ error: 'config load failed' }), {
        status: 503,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    if (url.endsWith('/api/v1/admin/configs/llm') && method === 'PUT') {
      putSpy();
    }

    if (url.endsWith('/api/v1/admin/test/llm') && method === 'POST') {
      postSpy();
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  expect(await screen.findByText('配置读取失败')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '测试连接' })).toBeDisabled();
  expect(screen.getByRole('button', { name: '保存配置' })).toBeDisabled();
  expect(putSpy).not.toHaveBeenCalled();
  expect(postSpy).not.toHaveBeenCalled();
});
