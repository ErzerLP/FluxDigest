import '@testing-library/jest-dom/vitest';
import { render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, test, vi } from 'vitest';

import { AppProviders } from '../app/providers/AppProviders';
import { defaultPromptTemplates } from '../pages/configs/prompts/defaultPromptTemplates';
import { PromptConfigPage } from '../pages/configs/prompts/PromptConfigPage';

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

function renderPage(ui: ReactElement) {
  return render(<AppProviders>{ui}</AppProviders>);
}

test('prompt config page loads editors and saves prompt payload', async () => {
  const putSpy = vi.fn();

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          prompts: {
            target_language: 'zh-CN',
            translation_prompt: 'translation-body',
            analysis_prompt: 'analysis-body',
            dossier_prompt: 'dossier-body',
            digest_prompt: 'digest-body',
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/configs/prompts') && method === 'PUT') {
      putSpy(JSON.parse(String(init?.body ?? '')));
      return new Response(JSON.stringify({ ok: true }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<PromptConfigPage />);

  expect(await screen.findByLabelText('Translation Prompt')).toHaveValue('translation-body');
  await userEvent.clear(screen.getByLabelText('Digest Prompt'));
  await userEvent.type(screen.getByLabelText('Digest Prompt'), 'digest prompt body');
  await userEvent.click(screen.getByRole('button', { name: '保存提示词' }));

  expect(putSpy).toHaveBeenCalledWith(
    expect.objectContaining({
      target_language: 'zh-CN',
      translation_prompt: 'translation-body',
      analysis_prompt: 'analysis-body',
      dossier_prompt: 'dossier-body',
      digest_prompt: 'digest prompt body',
    }),
  );
});

test('prompt config page can restore bundled defaults', async () => {
  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          prompts: {
            target_language: 'en',
            translation_prompt: 'custom-translation',
            analysis_prompt: 'custom-analysis',
            dossier_prompt: 'custom-dossier',
            digest_prompt: 'custom-digest',
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<PromptConfigPage />);

  await userEvent.click(await screen.findByRole('button', { name: '恢复默认' }));

  expect(screen.getByLabelText('Target Language')).toHaveValue(defaultPromptTemplates.target_language);
  expect(screen.getByLabelText('Translation Prompt')).toHaveValue(defaultPromptTemplates.translation_prompt);
  expect(screen.getByLabelText('Digest Prompt')).toHaveValue(defaultPromptTemplates.digest_prompt);
});
