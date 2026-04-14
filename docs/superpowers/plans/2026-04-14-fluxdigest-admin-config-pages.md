# FluxDigest Admin Config Pages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Miniflux / Prompts / Publish 三个 WebUI 配置页从占位壳层升级为可读写、可测试、可联调的真实业务页面。

**Architecture:** 以现有 `LLMConfigPage` 为交互母板，统一复用 `admin/configs` 快照、React Query、Mutation、`SecretField` 与 Alert 反馈机制；只在必要处补充前端 contract 和默认 prompt 恢复逻辑，不额外扩张后端范围。

**Tech Stack:** React, TypeScript, React Query, react-hook-form, Ant Design, Vitest, Testing Library, Go API contracts

---

### Task 1: 扩展 admin 前端 contract
- [ ] 补充 `web/src/types/admin.ts` 中 Miniflux / Publish / Prompt 更新类型。
- [ ] 补充 `web/src/services/api/admin.ts` 中 `updateMinifluxConfig`、`testMinifluxConfig`、`updatePublishConfig`、`testPublishConfig`、`updatePromptConfig`。
- [ ] 补充 `web/src/services/mutations/admin.ts` 中对应 mutation，并在成功后刷新 `configs/status`。

### Task 2: 实现 Miniflux 页面
- [ ] 先写 `web/src/__tests__/miniflux-config.test.tsx`，覆盖加载快照、保存配置、测试连接。
- [ ] 实现 `web/src/pages/configs/miniflux/MinifluxConfigPage.tsx` 的真实表单。
- [ ] 复用 SecretField 与 StatusBadge，展示接入状态与测试反馈。

### Task 3: 实现 Publish 页面
- [ ] 先写 `web/src/__tests__/publish-config.test.tsx`，覆盖 provider 切换、保存与测试。
- [ ] 实现 `web/src/pages/configs/publish/PublishConfigPage.tsx`。
- [ ] 支持 `halo` / `markdown_export` 的条件渲染与策略说明。

### Task 4: 实现 Prompt 页面
- [ ] 先写 `web/src/__tests__/prompt-config.test.tsx`，覆盖读取、保存、恢复默认。
- [ ] 实现 `web/src/pages/configs/prompts/PromptConfigPage.tsx`。
- [ ] 从 `configs/prompts/*.tmpl` 补一份默认 prompt 前端常量或静态资源镜像，供“恢复默认”使用。

### Task 5: 回归验证与联调
- [ ] 跑前端测试与构建。
- [ ] 跑 Go 全量测试，确认未破坏现有 contract。
- [ ] 推送 GitHub 分支。
- [ ] 测试服务器拉取分支，重建前端与服务，做真实 WebUI 点击验证。
- [ ] 验证通过后合并到 `master`。
