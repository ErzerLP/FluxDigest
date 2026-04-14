# 首次联调与使用

## 文档适用范围
适用于已经完成 FluxDigest/Miniflux/Halo 三件套部署、即将进行第一次链路联调的运维或产品负责人。文档讲的是 WebUI + API 操作，假定 FluxDigest WebUI（默认监听 18088 端口）和依赖服务都已可访问。

## 在 Miniflux 添加 RSS
**操作步骤**
1. 通过 FluxDigest Miniflux 配置页顶部的“打开 Miniflux 后台”按钮直接跳转到最近一次保存的 Miniflux Base URL，或在 Miniflux 主机上直接访问该 URL。
2. 在 Miniflux 后台点击“Feeds -> Add feed”填写订阅 URL/Title，选择分类并保存；FluxDigest 仅消费 Miniflux 端已抓取的文章，所有订阅管理（新增、修改、删除）必须始终在 Miniflux 后台执行。
3. 确保新增订阅能在 Miniflux 中看到最新文章条目，FluxDigest 再根据同步窗口拉取。

**关键字段**
- Miniflux 订阅 URL、分类、抓取间隔等均在 Miniflux 后台页面填写。

**校验结果**
- Miniflux 后台显示 Feed 有未读条目。
- FluxDigest 中的 Miniflux 配置页可以点击“测试连接”并显示成功提示（StatusBadge 会切换到 `configured` 状态）。

## 登录 FluxDigest WebUI
**操作步骤**
1. 在浏览器访问 FluxDigest WebUI（默认 `http://<主机>:18088`）。
2. 使用部署时配置的管理员账号登录；如果仍使用默认管理员 FluxDigest/FluxDigest，请参考部署文档中的安全说明，限制 WebUI 暴露范围并确保 APP_ADMIN_SESSION_SECRET 与 APP_SECRET_KEY 为高强度值。

**关键字段**
- 登录表单中的用户名与密码。

**校验结果**
- 登录成功后可以看到 `Jobs`、`Configs` 等菜单并进入下一步配置。不会在登录后直接进入只读页面。

## 配置 Miniflux
**操作步骤**
1. 进入 FluxDigest WebUI 的 `Configs -> Miniflux` 页面。
2. 填写 `Base URL`（指向 Miniflux 控制台，例如 `http://127.0.0.1:28082` 或局域网地址 `http://10.0.0.95:28082`）、`Fetch Limit`（一次拉取条数，默认 100）、`Lookback Hours`（回溯窗口，默认 24）。
3. 在 `Token` 区域选择“替换密钥”并输入 Miniflux 的 API Token（同样在 Miniflux 后台的 `Settings -> API` 中生成）后保存。
4. 点击“保存配置”，待页面上方出现“Miniflux 配置已保存”后，再点“测试连接”确认连通性。

**关键字段**
- `Base URL`：Miniflux 控制台地址。
- `Fetch Limit`：整数，至少 1。
- `Lookback Hours`：整数，至少 1。
- `Token`：SecretField（选择“替换密钥”后粘贴 token）。
- 按钮：`测试连接`（调用后端返回的状态置于 Alert），`保存配置`。

**校验结果**
- `StatusBadge` 显示 `configured`。
- `测试连接` Alert 显示 “连接测试：<status>”，描述中出现 `已收到测试结果`。如失败会有错误信息同时页面顶部会提示“连接测试失败”。

## 配置 LLM
**操作步骤**
1. 进入 `Configs -> LLM Config` 页面。
2. 填写 `Base URL`（如 LLM 提供商 API 地址）、`Model`（当前使用的模型 ID）、`Timeout (ms)`（建议 30000-45000 范围内）。
3. 在 `API Key` 区域切换到“替换密钥”并输入 LLM 提供商的 API Key，然后保存。
4. 保存后点击“测试连接”，需先把 API Key 切换到“替换密钥”并填写，否则页面会提示需要替换再测试。

**关键字段**
- `Base URL`、`Model`、`Timeout (ms)`。
- SecretField：API Key（首次配置必须填写；后续可保留现有 key）。
- 按钮：`测试连接`（此按钮会使用表单当前值发起连通性检测）、`保存配置`。

**校验结果**
- 上传保存后会出现 `配置已保存` 绿框提示。
- `测试连接` 成功后显示 Alert `连接测试：<status>`；失败会提示“连接测试失败”并在页面下方显示 `testGuidance` 提醒需要替换 key。

## 在 Halo 中创建 PAT
**操作步骤**
1. 登录 Halo 管理后台（例如 `http://<halo-host>:8090`）。
2. 进入用户设置或 IDE 栏找到“Personal Access Token/个人访问令牌”入口，新建一个 token，选择足够的作用域（通常是写入文章/发布权限），复制生成的 token。
3. 将 token 保存到安全的秘密管理系统，随后用于 FluxDigest 的发布配置。

**关键字段**
- Token 名称、作用域、有效期（根据 Halo 管理界面要求）。

**校验结果**
- Halo 显示 token 已创建，并提供一次性复制值；若刷新页面后看不到值，说明必须重新生成。

## 在 FluxDigest 配置 Halo 发布
**操作步骤**
1. 进入 `Configs -> Publish` 页面，确保 `Provider` 下拉选择 `Halo`。
2. 填写 `Halo Base URL`（如 `http://127.0.0.1:8090`）。
3. 在 `Halo Token` 区域选择“替换密钥”并粘贴上一步创建的 PAT。
4. 如果需要使用本地 Markdown 发布，可切换 `Provider` 为 `Markdown Export` 并填写 `Output Directory`。
5. 保存配置后点击“测试连接”，页面会提示当前连接状态。

**关键字段**
- `Provider`：`halo` 或 `markdown_export`（推荐 `halo`）。
- `Halo Base URL`、`Halo Token` SecretField；`Output Directory` 仅在 `markdown_export` 下显式显示。
- 按钮：`测试连接`、`保存配置`。

**校验结果**
- `StatusBadge` 显示 `configured`，`最近测试` 显示时间戳。
- `测试连接` 成功后弹出 `连接测试：<status>`，失败则 `发布通道测试失败` Alert 并描述错误。

## 手动触发日报
**操作步骤**
1. 目前 FluxDigest WebUI 的 `Jobs` 页面仅提供已运行 Job 的列表和 `查看详情` 按钮；不能直接在 UI 中触发每日 Digest。
2. 使用部署时设置的 `APP_JOB_API_KEY` 通过 API 调用 `/api/v1/jobs/daily-digest`（POST）触发。请求头需要 `X-API-Key: <APP_JOB_API_KEY>`。
3. 可选地在请求体中添加 `{"trigger_at": "2026-04-15T00:00:00Z"}` 让系统按指定日期调度。
4. 调用成功后可在 `Jobs` 页面刷新列表，点击 `查看详情`，对照 `status/trigger_source` 判断是否为手动触发。

**关键字段**
- API 请求头 `X-API-Key`。
- 可选的 `trigger_at`（ISO 8601 UTC）。

**校验结果**
- API 返回 `202 Accepted`（或 `200 Skipped` 如果相同 Digest 已排队）。
- `Jobs` 页面出现新的 `daily-digest` 记录，状态变为 `queued`/`running`/`completed`，并可点击查看详情。

## 用 API 验收
**操作步骤**
1. 先后访问以下 API：
   - `GET /api/v1/articles`：确认 FluxDigest 已经从 Miniflux 拉到文章列表。
   - `GET /api/v1/dossiers?limit=10`：查看当前活跃的 Dossier，验证文章翻译与核心摘要字段。
   - `GET /api/v1/digests/latest`：检查最新日报的 `content_markdown` 与 `publish_state`。
   - `POST /api/v1/jobs/daily-digest`：触发或重试日报（同前一节），然后再次刷新 `Jobs` 页面确认执行。
2. 所有接口均在 API 网关（默认 `http://localhost:18088`）下运行，没有额外 UI。

**关键字段**
- 无需额外字段，使用标准 HTTP GET/POST 即可；仅 `jobs/daily-digest` 需 `X-API-Key`。

**校验结果**
- `articles` 返回 `items` 数组不为空。
- `dossiers` 返回 `items` 包含 `publish_suggestion`、`core_summary` 等内容。
- `digests/latest` 返回 `publish_state`、`digest_date` 与预期时间一致。
- 手动触发 `jobs/daily-digest` 后 `Jobs` 页面出现新记录。

## 常见联调问题
1. **Miniflux Base URL 或 Token 写错**：保存后测试连接会报错 `Miniflux 连接测试失败`，状态仍是 `missing`，需要重新保存正确的 URL 或 Token。
2. **LLM API Key 未切换为“替换密钥”**：测试连接按钮会提示 `若要测试当前输入的新 key，请切换为“替换密钥”`，必须在 SecretField 中切换模式并填写后再测试。
3. **Halo Token 失效或权限不足**：`测试连接` 会抛出 `发布通道测试失败`，同时 StatusBadge 不会进入 `configured`。
4. **手动触发时未带 `X-API-Key`**：`/api/v1/jobs/daily-digest` 返回 `401 Invalid API key`。
5. **FluxDigest 无法读取文章**：访问 `/api/v1/articles` 如果返回 503，说明 Miniflux 未配置或抓取窗口不足，应回到 Miniflux 页面确认 Lookback Hours。

每次联调遇到问题时，优先检查对应配置页上的 Alert / StatusBadge，确认是否有 `Alert` 显示错误信息并根据提示修复后再重试。



