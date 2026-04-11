// @vitest-environment node
import { expect, test } from 'vitest';

import config from '../vite.config';

test('proxies /api requests to the local api server during development', () => {
  expect(config.server?.proxy?.['/api']).toMatchObject({
    target: 'http://127.0.0.1:8080',
    changeOrigin: true,
  });
});
