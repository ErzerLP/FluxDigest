import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, test, vi } from 'vitest';

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

test('jobs page opens detail drawer', async () => {
  fetchMock.mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;

    if (url.includes('/api/v1/admin/jobs/job-1')) {
      return new Response(
        JSON.stringify({
          id: 'job-1',
          status: 'succeeded',
          detail: { remote_url: 'https://blog.local/post/1' },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
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
});
