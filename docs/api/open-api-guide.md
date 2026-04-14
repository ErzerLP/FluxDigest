# FluxDigest 开放接口总览

本文档是 FluxDigest 的**人工可读开放接口总览**，用于帮助两类读者快速找到正确接口：

- 第三方集成开发者：优先关注文章、dossier、digest 等内容读取接口
- 自用 / 运维 / WebUI 对接者：在主文档基础上继续查看管理与运维附录

更细的字段定义、schema 与响应结构请以 `api/openapi/openapi.yaml` 为准。

## 文档边界

- 仅覆盖当前已经实现、且可从 OpenAPI 与现有实现确认存在的接口
- 主文档优先描述对外消费链路
- 管理、测试、任务触发接口统一收纳到附录
- 本文档是导航与总览，不替代 OpenAPI 规范

## API 基础约定

- **Base URL**：请将下文路径拼接到你的 FluxDigest API 服务地址，例如 `http://127.0.0.1:8080`
- **返回格式**：成功响应通常为 JSON，但部分错误响应可能没有响应体，最终以 `api/openapi/openapi.yaml` 为准
- **鉴权方式**：当前公开接口中，`POST /api/v1/jobs/daily-digest` 需要在请求头中传入 `X-API-Key`
- **时间格式**：日期字段通常使用 RFC 3339 的 `date` / `date-time` 形式
- **事实来源**：若本文档与 `api/openapi/openapi.yaml` 出现差异，以 OpenAPI 与实际实现为准

## 基础运行与观测接口

### 健康检查
- **接口**：`GET /healthz`
- **用途**：确认 API 进程是否存活
- **返回概览**：返回简单的健康状态对象，例如 `{"status":"ok"}`
- **适用场景**：反向代理探活、systemd / 容器健康检查、外部监控

### 指标导出
- **接口**：`GET /metrics`
- **用途**：导出 Prometheus 文本格式指标
- **返回概览**：文本内容，不是 JSON
- **适用场景**：Prometheus 抓取、基础运行观测

## 对外内容读取接口

### 文章列表
- **接口**：`GET /api/v1/articles`
- **用途**：读取已同步并处理过的文章列表，适合做文章流、归档页或二次分发入口
- **关键请求参数**：当前 OpenAPI 未声明 query 参数；默认直接读取文章列表
- **返回概览**：文章基础信息、来源信息、翻译结果、核心摘要、重要度等字段
- **使用建议**：如果需要单篇深度翻译/分析内容，请继续读取 dossier 接口

### dossier 列表
- **接口**：`GET /api/v1/dossiers`
- **用途**：拉取当前活跃的单篇文章解读结果列表
- **关键请求参数**：支持 `limit`，范围 `1-100`
- **返回概览**：dossier ID、关联文章 ID、译文标题、核心摘要、主题分类、推荐发布状态、发布状态等
- **使用建议**：适合做“今日值得读”“推荐发布候选”等上层聚合视图

### dossier 详情
- **接口**：`GET /api/v1/dossiers/{id}`
- **用途**：读取单篇文章的完整翻译、润色、分析与发布建议
- **关键请求参数**：路径参数 `id`
- **返回概览**：包含 `summary_polished`、`content_polished_markdown`、`analysis_longform_markdown`、`impact_analysis`、`publish_suggestion`、`publish_state` 等核心字段
- **使用建议**：适合博客、知识库、Bot、二次加工发布系统直接消费

### 最新日报
- **接口**：`GET /api/v1/digests/latest`
- **用途**：获取最新一篇每日汇总日报
- **关键请求参数**：无
- **返回概览**：日报日期、标题、副标题、Markdown/HTML 正文、关联条目列表、发布状态与远端地址
- **使用建议**：适合博客首页、消息推送、每日播报、对外摘要接口

## 运行配置 / 能力查询接口

### 当前生效 Profile
- **接口**：`GET /api/v1/profiles/{profileType}/active`
- **用途**：查看指定类型的当前生效配置版本
- **关键请求参数**：路径参数 `profileType`
- **返回概览**：`profile_type`、`name`、`version`、`is_active`、`payload`
- **使用建议**：如果外部系统需要理解当前启用的 LLM / Prompt / 发布策略，可以先读取这里确认活动配置

## 附录 A：管理与运维接口

> 以下接口主要面向 FluxDigest WebUI、部署联调与日常维护，不建议普通第三方内容消费方把这些管理接口作为公共集成协议依赖。

### 管理状态与配置读取
- `GET /api/v1/admin/status`：读取系统/运行时/集成状态概览（字段以 OpenAPI 为准）
- `GET /api/v1/admin/configs`：读取当前 LLM、Miniflux、Publish、Prompt 配置快照

### 配置写入接口
- `PUT /api/v1/admin/configs/llm`：更新 LLM 基础配置
- `PUT /api/v1/admin/configs/miniflux`：更新 Miniflux 基础配置
- `PUT /api/v1/admin/configs/publish`：更新发布器配置（字段以 OpenAPI 为准）
- `PUT /api/v1/admin/configs/prompts`：更新 translation / analysis / dossier / digest 提示词配置

### 联调与任务接口
- `POST /api/v1/admin/test/llm`：测试 LLM 连通性
- `GET /api/v1/admin/jobs`：查询最近的任务执行记录，支持 `limit`
- `POST /api/v1/jobs/daily-digest`：手动触发一次日报任务；该接口需要 `X-API-Key`

## 附录 B：常见错误与相关资源

### 常见错误类型
- **400**：请求体、query 参数或时间格式不合法
- **401**：`POST /api/v1/jobs/daily-digest` 的 `X-API-Key` 无效
- **500**：内部处理失败，例如配置更新失败、连通性测试失败、任务入队失败
- **503**：依赖未配置或相关服务不可用，导致请求无法正常处理

### 相关资源
- **机器可读 OpenAPI**：`api/openapi/openapi.yaml`
- **仓库入口文档**：`README.md`
- **部署与联调**：`docs/deployment/full-stack-ubuntu.md`、`docs/deployment/fluxdigest-systemd.md`、`docs/deployment/integration-setup.md`
