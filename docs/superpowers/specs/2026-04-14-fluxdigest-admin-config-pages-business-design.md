# FluxDigest 管理台配置页业务化设计

## 目标

把目前仍为占位的 3 个 WebUI 页面真正接入后台业务：

1. `Miniflux`：配置读写、接入状态、连通性测试、同步策略展示。
2. `Prompts`：翻译 / 分析 / dossier / digest 提示词的读取、编辑、保存、恢复默认。
3. `Publish`：发布通道选择、Provider 相关配置、输出策略、连通性测试。

## 当前事实

- `origin/master` 已具备后端 contract：
  - `GET /api/v1/admin/configs`
  - `PUT /api/v1/admin/configs/miniflux`
  - `PUT /api/v1/admin/configs/prompts`
  - `PUT /api/v1/admin/configs/publish`
  - `POST /api/v1/admin/test/miniflux`
  - `POST /api/v1/admin/test/publish`
- 前端目前只有 `LLMConfigPage` 是真实表单，其余三页仍是静态占位。
- Dashboard 的 `integrations.miniflux / integrations.publisher` 状态已经有后端来源，可以直接前端消费。

## 本轮范围

### Miniflux 页面
- 复用 `LLMConfigPage` 的交互模型：读取快照 → 表单编辑 → SecretField → 保存 → 连通性测试。
- 展示：Base URL、Fetch Limit、Lookback Hours、Token 是否已配置、最近测试状态。
- 不在本轮重做 Miniflux 的 RSS 订阅 CRUD，仍由 Miniflux 自身后台负责。

### Prompts 页面
- 暴露 4 个 prompt 编辑器：`translation` / `analysis` / `dossier` / `digest`。
- 支持 `target_language` 编辑。
- 增加“恢复默认”按钮：回退到项目内嵌 prompt 模板，而不是简单清空。
- 保存后刷新 `admin/configs` 快照，保证 worker 下一轮运行可读取最新版本。

### Publish 页面
- 支持 `halo` 与 `markdown_export` 两种 provider。
- `halo` 模式下展示：Base URL、Token。
- `markdown_export` 模式下展示：Output Directory。
- 统一保存 provider 与策略字段，支持连通性测试。
- 将 provider 与输出策略说明做成明确 UI，而不是模糊占位文案。

## 交互设计

- 页面顶部统一保留 `PageHeader`。
- 每页右上角统一放置：`测试连接` / `保存配置`。
- 成功、失败、提示信息均通过 `Alert` 呈现。
- 在 snapshot 未加载成功前，按钮禁用。
- Secret 字段继续使用 `SecretField`，保持和 LLM 页面一致。

## 测试策略

- 前端：为三个页面分别增加 Vitest + Testing Library 测试，覆盖加载、保存、测试连接、provider 切换、恢复默认。
- 后端：若发现 contract 缺口，再补 Go 测试；否则优先复用现有 contract。
- 最终验证：
  - `go test -p 1 ./... -count=1`
  - `npm --prefix web test -- --run`
  - `npm --prefix web run build`
  - 测试服务器拉取分支后进行真实页面联调。
