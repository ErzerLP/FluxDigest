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
  const schedulerSpy = vi.fn();

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
            article_publish_mode: 'suggested',
            article_review_mode: 'manual_review',
          },
          scheduler: {
            enabled: true,
            schedule_time: '07:30',
            timezone: 'Asia/Shanghai',
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

    if (url.endsWith('/api/v1/admin/configs/scheduler') && method === 'PUT') {
      schedulerSpy(JSON.parse(String(init?.body ?? '')));
      return new Response(JSON.stringify({ ok: true }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<PublishConfigPage />);

  expect(await screen.findByLabelText('Provider / Halo 发布')).toBeChecked();
  expect(screen.getByLabelText('Provider / Markdown 导出')).not.toBeChecked();
  expect(screen.getByLabelText('文章发布流程 / 只发日报')).not.toBeChecked();
  expect(screen.getByLabelText('文章发布流程 / 部分发送 + 审核')).toBeChecked();
  expect(screen.getByLabelText('文章发布流程 / 全部发送')).not.toBeChecked();
  expect(screen.getByLabelText('文章发布审核 / 人工审核')).toBeChecked();
  expect(screen.getByLabelText('文章发布审核 / 自动发布')).not.toBeChecked();
  expect(screen.getByLabelText('Halo Base URL')).toBeInTheDocument();
  expect(screen.getByLabelText('日报生成时间')).toHaveValue('07:30');

  await userEvent.click(screen.getByLabelText('Provider / Markdown 导出'));
  await userEvent.clear(screen.getByLabelText('日报生成时间'));
  await userEvent.type(screen.getByLabelText('日报生成时间'), '08:15');
  await userEvent.click(screen.getByLabelText('文章发布流程 / 全部发送'));
  await userEvent.click(screen.getByLabelText('文章发布审核 / 自动发布'));
  expect(screen.getByLabelText('Output Directory')).toBeInTheDocument();

  await userEvent.clear(screen.getByLabelText('Output Directory'));
  await userEvent.type(screen.getByLabelText('Output Directory'), '/data/digests');
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      provider: 'markdown_export',
      output_dir: '/data/digests',
      article_publish_mode: 'all',
      article_review_mode: 'auto_publish',
      halo_token: { mode: 'keep' },
    }),
  );
  expect(schedulerSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      enabled: true,
      schedule_time: '08:15',
      timezone: 'Asia/Shanghai',
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

test('publish config page can run daily digest manually', async () => {
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
            article_publish_mode: 'digest_only',
            article_review_mode: 'manual_review',
          },
          scheduler: {
            enabled: true,
            schedule_time: '07:00',
            timezone: 'Asia/Shanghai',
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

    if (url.endsWith('/api/v1/admin/jobs/daily-digest/run') && method === 'POST') {
      postSpy(JSON.parse(String(init?.body ?? '{}')));
      return new Response(JSON.stringify({ status: 'accepted', digest_date: '2026-04-15', force: true }), {
        status: 202,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<PublishConfigPage />);

  await userEvent.click(await screen.findByRole('button', { name: '手动生成日报' }));

  expect(postSpy).toHaveBeenCalledWith(expect.objectContaining({ force: true }));
  expect(await screen.findByText(/2026-04-15/)).toBeInTheDocument();
});
