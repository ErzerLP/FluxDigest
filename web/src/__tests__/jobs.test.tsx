import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { JobsPage } from '../pages/jobs/JobsPage';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderPage(ui: ReactElement) {
  return render(<AppProviders>{ui}</AppProviders>);
}

test('jobs page opens detail drawer from list detail without requesting future endpoint', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.includes('/api/v1/admin/jobs/job-1')) {
      throw new Error('detail endpoint should not be requested when list detail exists');
    }

    if (url.includes('/api/v1/admin/jobs')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'job-1',
              job_type: 'daily_digest_run',
              status: 'succeeded',
              digest_date: '2026-04-11',
              detail: { remote_url: 'https://blog.local/post/1' },
            },
          ],
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<JobsPage />);
  await userEvent.click(await screen.findByRole('button', { name: '查看详情' }));
  expect(await screen.findByText('https://blog.local/post/1')).toBeInTheDocument();
  expect(fetchMock).toHaveBeenCalledTimes(1);
});

test('jobs page surfaces detail request failure when future endpoint is unavailable', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.includes('/api/v1/admin/jobs/job-2')) {
      return new Response(JSON.stringify({ error: 'detail endpoint unavailable' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    if (url.includes('/api/v1/admin/jobs')) {
      return new Response(
        JSON.stringify({
          items: [
            {
              id: 'job-2',
              job_type: 'daily_digest_run',
              status: 'failed',
              digest_date: '2026-04-12',
            },
          ],
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<JobsPage />);
  await userEvent.click(await screen.findByRole('button', { name: '查看详情' }));
  expect(await screen.findByText('detail endpoint unavailable')).toBeInTheDocument();
});

test('jobs page shows list error state without empty state fallback', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.includes('/api/v1/admin/jobs')) {
      return new Response(JSON.stringify({ error: 'jobs list failed' }), {
        status: 503,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<JobsPage />);

  expect(await screen.findByText('任务读取失败')).toBeInTheDocument();
  expect(screen.queryByText('暂无任务运行记录。')).not.toBeInTheDocument();
});
