import { beforeEach, describe, expect, test, vi } from 'vitest';

import { AdminApiError, getAdminStatus } from './admin';

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
});
