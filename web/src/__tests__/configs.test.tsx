import '@testing-library/jest-dom/vitest';
import { render, screen, waitFor } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { LLMConfigPage } from '../pages/configs/llm/LLMConfigPage';

const fetchMock = vi.fn<typeof fetch>();
type adminTestGlobals = typeof globalThis & { __ADMIN_REQUEST_TIMEOUT_MS__?: number };

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  delete (globalThis as adminTestGlobals).__ADMIN_REQUEST_TIMEOUT_MS__;
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
  await userEvent.clear(screen.getByLabelText('Timeout (ms)'));
  await userEvent.type(screen.getByLabelText('Timeout (ms)'), '45000');
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      base_url: expect.stringContaining('https://proxy.local/v1'),
      model: 'gpt-4.1-mini',
      timeout_ms: 45000,
      api_key: { mode: 'keep' },
    }),
  );
  expect(putSpy).toHaveBeenCalledWith(
    expect.not.objectContaining({
      is_enabled: expect.anything(),
    }),
  );
  expect(screen.getByLabelText('Timeout (ms)')).toBeInTheDocument();
  expect(screen.queryByLabelText('启用 LLM')).not.toBeInTheDocument();
});

test('llm config page sends timeout_ms in connection test payload', async () => {
  const postSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          llm: {
            base_url: 'https://llm.local/v1',
            model: 'gpt-4.1-mini',
            timeout_ms: 30000,
            api_key: { is_set: true, masked_value: 'secr****' },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/test/llm') && method === 'POST') {
      const bodyText = typeof init?.body === 'string' ? init.body : '';
      postSpy(JSON.parse(bodyText));
      return new Response(JSON.stringify({ status: 'ok', message: 'ok' }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  await userEvent.click(await screen.findByText('替换密钥'));
  await userEvent.type(screen.getByLabelText('新 API Key'), 'test-token');
  await userEvent.clear(screen.getByLabelText('Timeout (ms)'));
  await userEvent.type(screen.getByLabelText('Timeout (ms)'), '45000');
  await userEvent.click(screen.getByRole('button', { name: '测试连接' }));

  expect(postSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      base_url: 'https://llm.local/v1',
      model: 'gpt-4.1-mini',
      timeout_ms: 45000,
      api_key: 'test-token',
    }),
  );
});

test('llm config page normalizes decimal timeout_ms before save', async () => {
  const putSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          llm: {
            base_url: 'https://llm.local/v1',
            model: 'gpt-4.1-mini',
            timeout_ms: 30000,
            api_key: { is_set: true, masked_value: 'secr****' },
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

  await userEvent.clear(await screen.findByLabelText('Timeout (ms)'));
  await userEvent.type(screen.getByLabelText('Timeout (ms)'), '45000.9');
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      timeout_ms: 45000,
    }),
  );
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
    await screen.findByText(
      '保留现有只会沿用已保存的 key；若要测试当前输入的新 key，请切换为“替换密钥”并输入待测 key。',
    ),
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

test('llm config page blocks save when replace mode has empty api key', async () => {
  const putSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

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

    if (url.endsWith('/api/v1/admin/configs/llm') && method === 'PUT') {
      putSpy();
      return new Response(JSON.stringify({ ok: true }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  await userEvent.click(await screen.findByText('替换密钥'));
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).not.toHaveBeenCalled();
  expect(await screen.findByText('当前处于“替换密钥”模式，请先输入 API key 再保存。')).toBeInTheDocument();
});

test('llm config page times out hanging connection test requests', async () => {
  (globalThis as adminTestGlobals).__ADMIN_REQUEST_TIMEOUT_MS__ = 20;

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

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

    if (url.endsWith('/api/v1/admin/test/llm') && method === 'POST') {
      return await new Promise<Response>(() => undefined);
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  const user = userEvent.setup();

  await user.click(await screen.findByText('替换密钥'));
  await user.type(screen.getByLabelText('新 API Key'), 'test-token');

  const testButton = screen.getByRole('button', { name: '测试连接' });
  await user.click(testButton);
  expect(testButton).toHaveClass('ant-btn-loading');

  expect(await screen.findByText('连接测试失败')).toBeInTheDocument();
  expect(await screen.findByText('请求超时，请检查代理、网络或服务地址后重试。')).toBeInTheDocument();
  await waitFor(() => expect(testButton).not.toHaveClass('ant-btn-loading'));
});

test('llm config page times out hanging save requests', async () => {
  (globalThis as adminTestGlobals).__ADMIN_REQUEST_TIMEOUT_MS__ = 20;

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

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

    if (url.endsWith('/api/v1/admin/configs/llm') && method === 'PUT') {
      return await new Promise<Response>(() => undefined);
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<LLMConfigPage />);

  const saveButton = await screen.findByRole('button', { name: '保存配置' });
  const user = userEvent.setup();

  await user.click(saveButton);
  expect(saveButton).toHaveClass('ant-btn-loading');

  expect(await screen.findByText('配置保存失败')).toBeInTheDocument();
  expect(await screen.findByText('请求超时，请检查代理、网络或服务地址后重试。')).toBeInTheDocument();
  await waitFor(() => expect(saveButton).not.toHaveClass('ant-btn-loading'));
});
