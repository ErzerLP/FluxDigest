# FluxDigest

FluxDigest 是面向个人自托管的 RSS 智能处理入口：
Miniflux ⟶ FluxDigest 消费聚合内容、调用 LLM 处理、生成每日 digest，再通过发布器输送到 Halo 或 Markdown。
FluxDigest 提供 API 与 WebUI 供运维/开发使用，但**不负责 RSS 订阅源管理**——这部分始终由 Miniflux 后台完成。

## FluxDigest 是什么 / 核心能力
FluxDigest 的核心职能是把 Miniflux 已抓取的文章，通过 AI 进行翻译、分析、摘要与每日汇总，并把成果交由发布器输出。目前默认发布目标是 Halo，也可以导出为 Markdown 文件或通过 API 供其他系统消费。

主要能力：
- LLM 翻译、分析、摘要与每日报告生成，支持主模型 + fallback chain，流程中可配置超时与重试策略。
- Dashboard / 配置 WebUI 与 OpenAPI 管理端点，便于在运行时调整 Miniflux、LLM、发布通道等参数。
- 多进程结构（API / Worker / Scheduler）+ systemd 脚本方便分离部署，Worker 负责文章处理、Scheduler 负责定时任务。

## 组件职责与边界
- Miniflux：管理 RSS 订阅源、抓取、分类，并提供已聚合文章给 FluxDigest。
- FluxDigest：消费 Miniflux 产出的文章，负责翻译/分析/摘要、调度每日 digest 并交付发布器；WebUI/API 仅用于配置与运维，未承担订阅管理。
- Halo：FluxDigest 当前默认推荐的发布目标，负责最终博客内容的渲染与呈现。

## 订阅管理说明
所有订阅的添加、分组或更新必须在 Miniflux 后台完成；FluxDigest 只处理 Miniflux 已采集的内容，用于后续翻译、分析、汇总与发布。

## 首次部署入门提示
在正式进入文档之前，请先准备一台 Ubuntu 22.04/24.04 服务器、PostgreSQL 与 Redis （可由部署文档指引安装），并确保 Miniflux、LLM 服务与 Halo 可达。简单来说，先把基础依赖部署起来，再阅读下文推荐的文档按顺序执行。

## 推荐阅读顺序（首次部署入口）
1. [docs/deployment/full-stack-ubuntu.md](docs/deployment/full-stack-ubuntu.md)：从服务器准备到 PostgreSQL/Redis、Miniflux、Halo 与 FluxDigest 一步步覆盖，帮助你搭起完整环境。
2. [docs/deployment/fluxdigest-systemd.md](docs/deployment/fluxdigest-systemd.md)：在完成依赖后，详读系统级部署与 systemd 管理，了解 env 文件、升级与回滚流程。
3. [docs/deployment/integration-setup.md](docs/deployment/integration-setup.md)：最后调通 Miniflux、LLM、Halo 与 FluxDigest 的联动，并验证日报/发布是否正常。
> 更多环境变量与配置细节请以 deploy/systemd/fluxdigest.env.example 与上述部署文档为准。

## 接口与集成
- [开放接口总览](docs/api/open-api-guide.md)：面向第三方集成与运维调试的人类可读接口导航。
- [OpenAPI 规范](api/openapi/openapi.yaml)：机器可读 schema、请求/响应与鉴权定义。
- [联调与配置指南](docs/deployment/integration-setup.md)：对接 Miniflux、LLM、Halo 与手动触发任务的操作说明。

## 开发 Smoke 与正式部署边界
- Smoke / 本地验证：deployments/compose/docker-compose.yml 仅提供 mock Miniflux/LLM 的基础依赖，用于开发、Smoke 测试或 CI 快速回归。
- 正式环境：请基于真实 Miniflux、真实 LLM、真实 Halo，并通过 deploy/scripts/deploy-systemd.sh 等脚本交由 systemd 管理 FluxDigest API/Worker/Scheduler。
> Compose 场景只是验证逻辑，不适合作为生产部署基础。

## 默认管理员与安全建议
FluxDigest 初次部署时默认管理员用户名/密码均为 FluxDigest（详见 internal/service/admin_user_service.go 默认用户逻辑）。当前版本会把该账户标记为 `must_change_password=true`，但尚未提供完整的密码修改 UI/API，因此上线前应限制 WebUI 暴露范围，并在 `deploy/systemd/fluxdigest.env.example` 中设置高强度的 `APP_ADMIN_SESSION_SECRET` 与 `APP_SECRET_KEY` 等安全密钥。

## 关键配置项概览
- APP_MINIFLUX_BASE_URL / APP_MINIFLUX_AUTH_TOKEN：指向 Miniflux 后台地址和 API Token。
- APP_LLM_BASE_URL / APP_LLM_API_KEY / APP_LLM_MODEL / APP_LLM_TIMEOUT_MS：外部 LLM 服务的入口、身份与超时控制。
- APP_PUBLISH_HALO_BASE_URL / APP_PUBLISH_HALO_TOKEN / APP_PUBLISH_CHANNEL：Halo 发布通道配置（推荐显式设置为 halo），包含服务地址与 PAT。
- APP_ADMIN_SESSION_SECRET：FluxDigest WebUI 管理会话签名密钥。
- APP_SECRET_KEY：通用安全签名与加密密钥。
> 以上配置示例可在 deploy/systemd/fluxdigest.env.example 查阅；完整配置清单请参照部署文档与该 env 文件。

## 快速访问入口
- FluxDigest WebUI：http://<host>:18088/
- Miniflux 示例入口：http://<host>:28082/
- Halo 示例入口：http://<host>:8090/

## 总结
本 README 作为首次部署用户的入口页，先理解各组件职责、默认凭据与关键配置，再按推荐顺序进入详细部署、systemd 以及联调文档。
