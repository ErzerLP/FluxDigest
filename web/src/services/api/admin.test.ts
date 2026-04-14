import { beforeEach, describe, expect, test, vi } from 'vitest';

import { AdminApiError, getAdminStatus, loginAdmin, testLLMConfig } from './admin';

const fetchMock = vi.fn<typeof fetch>();

describe('admin api client', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });

  test('surfaces backend error field from failed admin responses', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: 'admin status reader is not configured' }), {
        status: 503,
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );

    await expect(getAdminStatus()).rejects.toEqual(
      new AdminApiError('admin status reader is not configured', 503),
    );
  });

  test('testLLMConfig uses provided timeout for client deadline', async () => {
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');

    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ status: 'ok', message: 'ok' }), {
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );

    try {
      await testLLMConfig({
        base_url: 'https://llm.local/v1',
        model: 'gpt-4.1-mini',
        api_key: 'token',
        timeout_ms: 45000,
      });

      expect(setTimeoutSpy).toHaveBeenCalled();
      expect(setTimeoutSpy.mock.calls[0]?.[1]).toBe(45000);
    } finally {
      setTimeoutSpy.mockRestore();
    }
  });

  test('testLLMConfig fallbacks to at least 30000ms when timeout is missing', async () => {
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');

    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ status: 'ok', message: 'ok' }), {
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );

    try {
      await testLLMConfig({
        base_url: 'https://llm.local/v1',
        model: 'gpt-4.1-mini',
        api_key: 'token',
      });

      expect(setTimeoutSpy).toHaveBeenCalled();
      expect(setTimeoutSpy.mock.calls[0]?.[1]).toBeGreaterThanOrEqual(30000);
    } finally {
      setTimeoutSpy.mockRestore();
    }
  });

  test('testLLMConfig caps oversized timeout values to a safe upper bound', async () => {
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');

    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ status: 'ok', message: 'ok' }), {
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );

    try {
      await testLLMConfig({
        base_url: 'https://llm.local/v1',
        model: 'gpt-4.1-mini',
        api_key: 'token',
        timeout_ms: 3_000_000_000,
      });

      expect(setTimeoutSpy).toHaveBeenCalled();
      expect(setTimeoutSpy.mock.calls[0]?.[1]).toBe(2_147_483_647);
    } finally {
      setTimeoutSpy.mockRestore();
    }
  });

  test('admin requests include credentials for cookie-based session auth', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ system: { api: 'ok' } }), {
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );

    await getAdminStatus();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/status',
      expect.objectContaining({
        credentials: 'include',
      }),
    );
  });

  test('loginAdmin posts credentials to auth endpoint', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ user_id: 'admin-1', username: 'FluxDigest' }), {
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );

    await loginAdmin({ username: 'FluxDigest', password: 'FluxDigest' });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/auth/login',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
      }),
    );
  });
});
