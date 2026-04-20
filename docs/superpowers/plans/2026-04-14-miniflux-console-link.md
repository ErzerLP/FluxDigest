# Miniflux Console Link Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 FluxDigest 的 `Miniflux` 配置页增加一个“打开 Miniflux 后台”入口，用户可直接跳到 Miniflux 原生 WebUI。

**Architecture:** 仅修改前端 `MinifluxConfigPage`，复用当前已加载的 `base_url` 作为跳转目标，在页面右上角操作区新增按钮，不引入新的后端接口。测试层通过 Vitest 验证按钮展示、点击后调用 `window.open`，并保持现有保存/测试流程不受影响。

**Tech Stack:** React, TypeScript, Ant Design, Vitest, Testing Library

---

### Task 1: 在 Miniflux 配置页增加后台跳转入口

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\admin-config-pages-live\web\src\pages\configs\miniflux\MinifluxConfigPage.tsx`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\admin-config-pages-live\web\src\__tests__\miniflux-config.test.tsx`

- [ ] **Step 1: 写一个失败测试，验证按钮会打开当前 Base URL**

```tsx
test('miniflux config page can open miniflux console in new tab', async () => {
  const openSpy = vi.fn();
  vi.stubGlobal('open', openSpy);

  fetchMock.mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method =
      init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET');

    if (url.endsWith('/api/v1/admin/configs') && method === 'GET') {
      return new Response(
        JSON.stringify({
          miniflux: {
            base_url: 'http://127.0.0.1:28082',
            fetch_limit: 100,
            lookback_hours: 24,
            api_token: { is_set: true, masked_value: 'mini****' },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    if (url.endsWith('/api/v1/admin/status') && method === 'GET') {
      return new Response(
        JSON.stringify({
          integrations: {
            miniflux: {
              configured: true,
              last_test_status: 'ok',
            },
          },
        }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    return new Response('not found', { status: 404 });
  });

  renderPage(<MinifluxConfigPage />);

  await userEvent.click(await screen.findByRole('button', { name: '打开 Miniflux 后台' }));

  expect(openSpy).toHaveBeenCalledWith(
    'http://127.0.0.1:28082',
    '_blank',
    'noopener,noreferrer',
  );
});
```

- [ ] **Step 2: 跑单测，确认它先失败**

Run:

```bash
npm --prefix web test -- --run src/__tests__/miniflux-config.test.tsx
```

Expected: FAIL，提示找不到“打开 Miniflux 后台”按钮或 `window.open` 未被调用。

- [ ] **Step 3: 在页面右上角操作区增加最小实现**

```tsx
const minifluxConsoleURL = currentConfig?.base_url?.trim() ?? '';

<Button
  onClick={() => window.open(minifluxConsoleURL, '_blank', 'noopener,noreferrer')}
  disabled={!configReady || !minifluxConsoleURL}
>
  打开 Miniflux 后台
</Button>
```

- [ ] **Step 4: 重跑该测试，确认通过**

Run:

```bash
npm --prefix web test -- --run src/__tests__/miniflux-config.test.tsx
```

Expected: PASS。

- [ ] **Step 5: 跑前端全量测试与构建**

Run:

```bash
npm --prefix web test -- --run
npm --prefix web run build
```

Expected: 全部通过，build 仅允许保留现有 chunk size warning。

- [ ] **Step 6: 跑 Go 全量测试，确认未影响其他模块**

Run:

```bash
go test -p 1 ./... -count=1
```

Expected: PASS。

- [ ] **Step 7: 提交并推送功能分支**

```bash
git add web/src/pages/configs/miniflux/MinifluxConfigPage.tsx web/src/__tests__/miniflux-config.test.tsx docs/superpowers/plans/2026-04-14-miniflux-console-link.md
git commit -m "feat: add miniflux console shortcut"
git push -u origin codex/miniflux-console-link
```

- [ ] **Step 8: 测试服拉取分支并部署验证**

Run:

```bash
cd /home/hjx/FluxDigest-admin-config-pages-live
git fetch origin
git checkout codex/miniflux-console-link
git reset --hard origin/codex/miniflux-console-link
sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest --env-file /etc/fluxdigest/fluxdigest.env --service-dir /etc/systemd/system
```

Expected: 服务健康检查通过；WebUI `Miniflux` 页面可见“打开 Miniflux 后台”按钮，点击后跳转到 Miniflux 原生后台。
