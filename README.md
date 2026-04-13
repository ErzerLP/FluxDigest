# FluxDigest

FluxDigest 是一个基于 Go 的 RSS 智能处理平台：从 Miniflux 拉取内容，使用 LLM 做翻译/分析，总结成每日日报，并通过发布器输出到外部渠道或本地文件。

它同时提供 API + WebUI，支持运行时配置管理，便于后续扩展更多模型、发布渠道和集成接口。

## 1) 项目介绍

### FluxDigest 是什么

- 面向个人/小团队的 RSS 内容处理与日报发布平台
- 主流程：`Miniflux -> AI 处理 -> 日报聚合 -> 发布`
- 采用 `rss-api`、`rss-worker`、`rss-scheduler` 三进程拆分，便于独立扩缩/部署

### 核心能力

- Miniflux 文章抓取与入库
- 基于 OpenAI-compatible 接口的翻译、分析、摘要生成
- 每日汇总日报（默认按 Asia/Shanghai 调度）
- 多发布通道（当前内置 `halo`、`holo` 与 `markdown_export`）
- API 查询与任务触发
- WebUI 管理（当前已完整接入 LLM 配置页，其余配置页为占位壳层）

### 适用场景

- 用 Miniflux 做订阅聚合，希望自动产出中文日报
- 需要把 AI 处理结果沉淀到 DB，并通过 API 对外提供
- 需要可持续扩展的接口/发布器生态（新增适配器而非重写主流程）

## 2) 系统架构 / 核心组件

```text
Miniflux --> rss-worker --> LLM
                   |         |
                   v         v
               PostgreSQL   Prompts
                   |
                   v
               Publisher ----> Halo / Holo / Markdown 文件

rss-scheduler --> Asynq(Redis) --> rss-worker
rss-api -------> DB + Asynq + WebUI(静态资源)
```

核心组件：

- `rss-api`：HTTP API、OpenAPI 对应接口、管理端接口、静态 WebUI 托管
- `rss-worker`：任务消费、文章处理、日报生成、发布执行
- `rss-scheduler`：定时触发日报任务
- `Miniflux`：RSS 聚合源
- `LLM`：翻译/分析/日报规划（支持主模型 + fallback model chain）
- `Publisher`：发布适配层（halo / holo / markdown_export）

## 3) 主要功能清单

- 健康检查与指标：`/healthz`、`/metrics`
- 文章查询：`/api/v1/articles`
- Dossier 查询：`/api/v1/dossiers`、`/api/v1/dossiers/{id}`
- 最新日报查询：`/api/v1/digests/latest`
- Profile 查询：`/api/v1/profiles/{profileType}/active`
- 管理接口：
  - `/api/v1/admin/status`
  - `/api/v1/admin/configs`
  - `/api/v1/admin/configs/llm`（更新 LLM）
  - `/api/v1/admin/test/llm`（连通性测试）
  - `/api/v1/admin/jobs`
- 任务触发（需 `X-API-Key`）：
  - `POST /api/v1/jobs/daily-digest`
  - `POST /api/v1/jobs/article-reprocess`

## 4) 快速开始（本地开发）

### 4.1 准备依赖

- Go `1.24+`
- Node.js `22+`
- PostgreSQL
- Redis

### 4.2 配置

```bash
cp configs/config.example.yaml configs/config.yaml
```

或直接使用环境变量（推荐，优先级高于 YAML）。

### 4.3 启动基础依赖（可选：用 compose 只拉基础服务）

```bash
docker compose -f deployments/compose/docker-compose.yml up -d postgres redis
```

### 4.4 启动后端三进程

```bash
# API
make run-api

# Worker
make run-worker

# Scheduler
make run-scheduler
```

### 4.5 启动 WebUI（开发模式）

```bash
npm --prefix web ci
npm --prefix web run dev
```

默认通过 Vite 代理到 `http://127.0.0.1:8080`。

### 4.6 一键 smoke（Windows PowerShell）

```powershell
./scripts/smoke-compose.ps1
```

## 5) 配置说明（重点）

配置读取顺序：`默认值 -> configs/config.yaml -> APP_* 环境变量覆盖`

关键环境变量：

- 基础
  - `APP_HTTP_PORT`
  - `APP_DATABASE_DSN`
  - `APP_REDIS_ADDR`
  - `APP_JOB_API_KEY`
  - `APP_JOB_QUEUE`
  - `APP_WORKER_CONCURRENCY`
- Miniflux
  - `APP_MINIFLUX_BASE_URL`
  - `APP_MINIFLUX_AUTH_TOKEN`
- LLM
  - `APP_LLM_BASE_URL`
  - `APP_LLM_API_KEY`
  - `APP_LLM_MODEL`
  - `APP_LLM_FALLBACK_MODELS`（逗号分隔）
  - `APP_LLM_TIMEOUT_MS`
- 发布
  - `APP_PUBLISH_CHANNEL`（`halo` / `holo` / `markdown` / `markdown_export`）
  - `APP_PUBLISH_HALO_BASE_URL`
  - `APP_PUBLISH_HALO_TOKEN`
  - `APP_PUBLISH_HOLO_ENDPOINT`
  - `APP_PUBLISH_HOLO_TOKEN`
  - `APP_PUBLISH_OUTPUT_DIR`
- WebUI 静态资源托管
  - `APP_STATIC_DIR`

参考：`configs/config.example.yaml`、`deploy/systemd/fluxdigest.env.example`。

### 5.1 Halo 官方发布器说明

- `channel=halo`：走 Halo 官方 Console API
- 建议迁移时**显式设置** `APP_PUBLISH_CHANNEL=halo`，不要依赖空值自动选择，避免被遗留的 `APP_PUBLISH_OUTPUT_DIR` / 旧发布配置误导
- 需要配置：
  - `APP_PUBLISH_HALO_BASE_URL`，例如 `http://127.0.0.1:8090`
  - `APP_PUBLISH_HALO_TOKEN`，即 Halo Personal Access Token
- 当前实现链路：
  - `POST /apis/api.console.halo.run/v1alpha1/posts`
  - `PUT /apis/api.console.halo.run/v1alpha1/posts/{name}/publish`
- `RemoteID` 保存 Halo `metadata.name`
- `RemoteURL` 保存 Halo `status.permalink`
- 当前策略：
  - `metadata.name` 使用内部唯一值 `fluxdigest-<unixnano>`
  - `spec.slug` 使用 ASCII 化后的日报别名，例如 `daily-digest-20260413-070000`

### 5.2 Holo 兼容发布器说明

- `channel=holo`：保留给旧的自定义 Holo 风格端点
- 需要配置：
  - `APP_PUBLISH_HOLO_ENDPOINT`
  - `APP_PUBLISH_HOLO_TOKEN`

## 6) 部署教程

### 6.1 普通开发 / 手动运行

适合本地或临时环境：

```bash
# 方式 A：直接运行源码（开发期最常用）
make run-api
make run-worker
make run-scheduler

# 方式 B：先编译，再运行产物（文件名按平台变化）
make build
./rss-api
./rss-worker
./rss-scheduler
```

### 6.2 systemd 正式部署

使用脚本：`deploy/scripts/deploy-systemd.sh`

默认行为：

1. 构建 Go + Web 产物（可 `--skip-build` 跳过）
2. 安装到 `${APP_ROOT}/releases/<release_id>`
3. 切换 `${APP_ROOT}/current` 软链
4. 渲染并安装三个 service unit
5. `systemctl daemon-reload && enable --now && restart`
6. 健康检查（默认 `http://127.0.0.1:${APP_HTTP_PORT}/healthz`）

示例：

```bash
sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest
```

### 6.3 升级脚本

`deploy/scripts/upgrade-systemd.sh` 是标准升级入口，本质透传到 `deploy-systemd.sh`。

```bash
sudo ./deploy/scripts/upgrade-systemd.sh
sudo ./deploy/scripts/upgrade-systemd.sh --skip-build --release-retention 7
```

### 6.4 回滚脚本

使用 `deploy/scripts/rollback-systemd.sh`：

- 不带参数：自动回滚到上一个可用 release
- `--release-id <id>`：回滚到指定 release

```bash
sudo ./deploy/scripts/rollback-systemd.sh
sudo ./deploy/scripts/rollback-systemd.sh --release-id 20260413093000
```

### 6.5 release 保留/自动清理

- 部署脚本默认 `RELEASE_RETENTION=5`
- 仅保留最近 N 个数字命名 release（`YYYYMMDDHHMMSS`）
- 自动跳过当前 `current` 指向版本
- `RELEASE_RETENTION=0` 可关闭自动清理

## 7) WebUI 用途说明

WebUI 路由位于 `web/src/app/router/index.tsx`，当前包含：

- Dashboard：系统概览、集成状态、最近任务摘要
- LLM Config：可读取/更新 LLM 配置，支持在线连通性测试
- Jobs：查看任务记录
- Miniflux / Prompts / Publish：目前是占位壳层页面（用于后续接入配置能力）

生产环境中，API 可通过 `APP_STATIC_DIR` 直接托管前端构建产物。

## 8) API / OpenAPI 与扩展能力

- OpenAPI 文件：`api/openapi/openapi.yaml`
- API 前缀：`/api/v1`
- 管理与任务触发接口已按契约暴露

扩展能力建议：

- 新发布渠道：实现 `internal/adapter/publisher.Publisher` 接口并在 `buildPublisher` 中注册
- 新模型接入：复用 `internal/adapter/llm` 工厂与 runtime profile
- 新运行时配置：沿用 profile 版本化机制（`internal/domain/profile` + repository/service）

## 9) 项目目录结构（简版）

```text
.
├─ api/openapi/                # OpenAPI 契约
├─ cmd/
│  ├─ rss-api/                 # API 入口
│  ├─ rss-worker/              # Worker 入口
│  └─ rss-scheduler/           # Scheduler 入口
├─ configs/
│  ├─ config.example.yaml
│  └─ prompts/                 # 默认提示词模板
├─ deploy/
│  ├─ scripts/                 # systemd deploy / upgrade / rollback
│  └─ systemd/                 # unit 模板与 env 示例
├─ deployments/
│  ├─ compose/
│  └─ docker/
├─ internal/
│  ├─ adapter/                 # Miniflux/LLM/Publisher 适配
│  ├─ app/                     # API/Worker/Scheduler 组装
│  ├─ repository/              # PostgreSQL 持久化
│  ├─ service/                 # 业务服务
│  ├─ workflow/                # 工作流
│  └─ task/asynq/              # 队列任务定义
├─ migrations/                 # DB migration
└─ web/                        # React + Vite WebUI
```
