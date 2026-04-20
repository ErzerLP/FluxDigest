# FluxDigest README & Deployment Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重写 `README.md` 并补齐 Ubuntu 全栈部署、FluxDigest systemd 运维、首次联调三篇文档，让首次部署用户可以从仓库首页走到完整上线与验收。

**Architecture:** 文档采用“README 作为总入口 + 分层部署文档”的结构。README 负责项目说明、职责边界、推荐阅读路径；`docs/deployment/` 下的 3 篇文档分别承载完整环境部署、FluxDigest 正式部署运维、首次接入联调，避免信息混杂。

**Tech Stack:** Markdown, Git, PowerShell, ripgrep (`rg`), 现有 FluxDigest 配置/脚本/路由实现, Miniflux/Halo 官方安装文档

---

## File Structure

- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\README.md`
  - 仓库首页入口，说明产品定位、职责边界、快速导航、部署入口。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\full-stack-ubuntu.md`
  - Ubuntu 22.04 / 24.04 下的首次完整部署主教程。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\fluxdigest-systemd.md`
  - FluxDigest 三个 systemd 服务的正式部署、升级、回滚、日志与 release 管理说明。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\integration-setup.md`
  - Miniflux / LLM / Halo / WebUI 的首次联调与验收步骤。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\scripts\deploy-systemd.sh`
  - 文档核对对象：部署脚本行为与参数。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\scripts\upgrade-systemd.sh`
  - 文档核对对象：升级入口。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\scripts\rollback-systemd.sh`
  - 文档核对对象：回滚流程。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\systemd\fluxdigest.env.example`
  - 文档核对对象：环境变量名、默认值与安全提示。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\internal\service\admin_user_service.go`
  - 文档核对对象：默认管理员账号密码事实来源。
- `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\api\openapi\openapi.yaml`
  - 文档核对对象：任务触发与查询接口路径。

### Task 1: 重构 README 为首次部署入口

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\README.md`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\systemd\fluxdigest.env.example`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\internal\service\admin_user_service.go`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\api\openapi\openapi.yaml`

- [ ] **Step 1: 用新的首页结构替换 README 顶部导航与定位说明**

```md
# FluxDigest

FluxDigest 是一个面向个人自托管场景的 RSS 智能处理平台：

`Miniflux -> FluxDigest -> LLM -> 每日汇总 -> Halo / Markdown`

它不负责 RSS 订阅管理本身，而是消费 Miniflux 已聚合的文章，完成翻译、分析、汇总与发布。

## 你可以用 FluxDigest 做什么
- 每天定时生成一篇“每日汇总日报”
- 保存每篇文章的翻译/分析结果，并通过 API 对外提供
- 用 WebUI 管理 LLM、Miniflux、Prompt、发布器配置
- 把日报推送到 Halo，或导出为 Markdown
```

- [ ] **Step 2: 增加职责边界、访问入口与推荐阅读路径**

```md
## 组件职责边界
- Miniflux：管理 RSS 订阅源、抓取聚合文章
- FluxDigest：翻译、分析、汇总、调度、发布
- Halo：博客发布目标之一

## 推荐阅读顺序
1. 首次部署：`docs/deployment/full-stack-ubuntu.md`
2. 仅部署 FluxDigest：`docs/deployment/fluxdigest-systemd.md`
3. 接通 Miniflux / LLM / Halo：`docs/deployment/integration-setup.md`
```

- [ ] **Step 3: 增加部署形态说明与当前能力边界**

```md
## 当前推荐部署方式
- 开发 / smoke：`deployments/compose/docker-compose.yml`
- 正式环境：真实 Miniflux + 真实 Halo + `deploy/scripts/deploy-systemd.sh`

> 注意：仓库内 compose 主要用于开发验证，包含 mock 组件，不等于生产一键全家桶。
```

- [ ] **Step 4: 增加 API / WebUI / 默认管理员说明**

```md
## 默认访问入口
- FluxDigest WebUI：`http://<host>:18088/`
- Miniflux：`http://<host>:28082/`（示例）
- Halo：`http://<host>:8090/`（示例）

## 首次登录
- FluxDigest 默认管理员用户名：`FluxDigest`
- FluxDigest 默认管理员密码：`FluxDigest`
- 首次登录后请立即修改，并替换 `APP_ADMIN_SESSION_SECRET` 与 `APP_SECRET_KEY`
```

- [ ] **Step 5: 运行 README 事实校验命令**

Run: `rg -n "Holo|HOLO" README.md ; rg -n "APP_PUBLISH_HALO_BASE_URL|APP_PUBLISH_HALO_TOKEN|APP_ADMIN_SESSION_SECRET|APP_SECRET_KEY" README.md ; rg -n "FluxDigest" internal/service/admin_user_service.go`
Expected: README 中不再出现 `Holo`；环境变量名与当前实现一致；默认管理员信息能在实现中找到事实来源。

- [ ] **Step 6: Commit**

```bash
git add README.md
git commit -m "docs: rewrite readme as deployment entrypoint"
```

### Task 2: 编写 Ubuntu 完整环境部署主教程

**Files:**
- Create: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\full-stack-ubuntu.md`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\systemd\fluxdigest.env.example`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\superpowers\specs\2026-04-14-fluxdigest-readme-and-deployment-docs-design.md`

- [ ] **Step 1: 写出文档骨架与目标说明**

```md
# FluxDigest 完整环境部署指南（Ubuntu 22.04 / 24.04）

## 本文适用范围
- 单机 / 单 VPS
- 个人自用
- Ubuntu 22.04 / 24.04 x86_64

## 最终你会得到什么
- PostgreSQL
- Redis
- Miniflux
- Halo
- FluxDigest API / Worker / Scheduler / WebUI
```

- [ ] **Step 2: 写基础依赖、端口规划与服务器准备章节**

```md
## 建议端口规划
- FluxDigest WebUI / API：18088
- Miniflux：28082
- Halo：8090
- PostgreSQL：5432
- Redis：6379

## 基础依赖
sudo apt update
sudo apt install -y curl git ca-certificates gnupg lsb-release
```

- [ ] **Step 3: 写 Miniflux 与 Halo 部署章节，明确采用官方推荐路径**

```md
## 部署 Miniflux
- 按 Miniflux 官方 Docker 文档准备 `docker-compose.yml`
- 初始化管理员账号
- 登录后台后添加 RSS 订阅源

## 部署 Halo
- 按 Halo 官方 Docker Compose 文档准备站点
- 完成初始化向导
- 创建管理员账号与 Personal Access Token
```

- [ ] **Step 4: 写 FluxDigest 正式部署章节，使用现有 systemd 脚本**

```md
## 部署 FluxDigest
cd /home/<user>
git clone https://github.com/ErzerLP/FluxDigest.git
cd FluxDigest

sudo ./deploy/scripts/deploy-systemd.sh \
  --app-root /opt/fluxdigest \
  --env-file /etc/fluxdigest/fluxdigest.env \
  --service-dir /etc/systemd/system
```

- [ ] **Step 5: 写首次登录、配置与验收章节**

```md
## 首次登录 FluxDigest WebUI
- 打开 `http://<host>:18088/`
- 用户名：`FluxDigest`
- 密码：`FluxDigest`

## 首次验收
- `curl http://127.0.0.1:18088/healthz`
- `systemctl status fluxdigest-api fluxdigest-worker fluxdigest-scheduler`
```

- [ ] **Step 6: 运行完整部署文档校验命令**

Run: `rg -n "Holo|TODO|TBD" docs/deployment/full-stack-ubuntu.md ; rg -n "deploy-systemd.sh|upgrade-systemd.sh|rollback-systemd.sh" docs/deployment/full-stack-ubuntu.md ; rg -n "APP_PUBLISH_HALO_BASE_URL|APP_PUBLISH_HALO_TOKEN" docs/deployment/full-stack-ubuntu.md`
Expected: 文档内没有占位词或旧命名；脚本名与环境变量名准确。

- [ ] **Step 7: Commit**

```bash
git add docs/deployment/full-stack-ubuntu.md
git commit -m "docs: add ubuntu full stack deployment guide"
```

### Task 3: 编写 FluxDigest systemd 正式部署与运维文档

**Files:**
- Create: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\fluxdigest-systemd.md`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\scripts\deploy-systemd.sh`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\scripts\upgrade-systemd.sh`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\scripts\rollback-systemd.sh`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\systemd\fluxdigest.env.example`

- [ ] **Step 1: 写目录结构与 env 文件章节**

```md
# FluxDigest systemd 部署与运维

## 安装目录结构
- `/opt/fluxdigest/releases/<timestamp>`
- `/opt/fluxdigest/current`
- `/etc/fluxdigest/fluxdigest.env`
- `/etc/systemd/system/fluxdigest-api.service`
- `/etc/systemd/system/fluxdigest-worker.service`
- `/etc/systemd/system/fluxdigest-scheduler.service`
```

- [ ] **Step 2: 写首次部署、升级、回滚命令章节**

```md
## 首次部署
sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest

## 标准升级
sudo ./deploy/scripts/upgrade-systemd.sh

## 回滚
sudo ./deploy/scripts/rollback-systemd.sh
sudo ./deploy/scripts/rollback-systemd.sh --release-id 20260414161113
```

- [ ] **Step 3: 写 release 清理、服务控制、日志排查章节**

```md
## release 清理
- 默认保留最近 5 个 release
- 自动跳过 current 指向版本
- 可用 `--release-retention 0` 关闭自动清理

## 常用命令
systemctl status fluxdigest-api fluxdigest-worker fluxdigest-scheduler
journalctl -u fluxdigest-api -n 200 --no-pager
readlink -f /opt/fluxdigest/current
```

- [ ] **Step 4: 写配置安全章节**

```md
## 必须修改的敏感配置
- `APP_JOB_API_KEY`
- `APP_MINIFLUX_AUTH_TOKEN`
- `APP_LLM_API_KEY`
- `APP_PUBLISH_HALO_TOKEN`
- `APP_ADMIN_SESSION_SECRET`
- `APP_SECRET_KEY`
```

- [ ] **Step 5: 运行 systemd 文档校验命令**

Run: `Test-Path deploy/scripts/deploy-systemd.sh ; Test-Path deploy/scripts/upgrade-systemd.sh ; Test-Path deploy/scripts/rollback-systemd.sh ; rg -n "release-retention|APP_ADMIN_SESSION_SECRET|APP_SECRET_KEY" docs/deployment/fluxdigest-systemd.md`
Expected: 3 个脚本文件都存在；文档中包含 release 清理与安全密钥说明。

- [ ] **Step 6: Commit**

```bash
git add docs/deployment/fluxdigest-systemd.md
git commit -m "docs: add systemd deployment and ops guide"
```

### Task 4: 编写首次联调与使用文档

**Files:**
- Create: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\integration-setup.md`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\api\openapi\openapi.yaml`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\internal\service\admin_user_service.go`
- Inspect: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\deploy\systemd\fluxdigest.env.example`

- [ ] **Step 1: 写 Miniflux 接入与订阅管理章节**

```md
# FluxDigest 首次联调与使用

## 1. 在 Miniflux 中添加 RSS
- 登录 Miniflux 后台
- 添加订阅源 / 分类
- 确认 Miniflux 中已经能看到新文章

> FluxDigest 不负责维护 RSS 订阅本身；订阅源管理始终在 Miniflux 中完成。
```

- [ ] **Step 2: 写 FluxDigest WebUI 配置 Miniflux / LLM / Prompt / Publish 的步骤**

```md
## 2. 登录 FluxDigest WebUI
- 地址：`http://<host>:18088/`
- 默认用户名：`FluxDigest`
- 默认密码：`FluxDigest`

## 3. 配置 Miniflux
- Base URL
- API Token
- Fetch Limit
- Lookback Hours
- 测试连接

## 4. 配置 LLM
- Base URL
- API Key
- Model
- Timeout
- 测试连接
```

- [ ] **Step 3: 写 Halo PAT 获取与发布配置步骤**

```md
## 5. 配置 Halo 发布
- 在 Halo 后台创建 Personal Access Token
- 在 FluxDigest Publish 页面填写：
  - Channel=`halo`
  - Halo Base URL
  - Halo Token
- 保存并测试连接
```

- [ ] **Step 4: 写手动重跑日报与验收接口章节**

```md
## 6. 手动重跑日报
- 在 Jobs 页面点击“手动重跑日报”
- 或调用 `POST /api/v1/jobs/daily-digest`

## 7. 验证结果
- `GET /api/v1/digests/latest`
- `GET /api/v1/dossiers`
- `GET /api/v1/articles`
```

- [ ] **Step 5: 写常见失败排查章节**

```md
## 8. 常见问题
- LLM 测试一直转圈：检查 Base URL、超时、代理、TLS
- Miniflux 连接失败：检查 Base URL、Token、端口是否可达
- Halo 发布失败：检查 PAT、站点地址、接口权限
- 有文章无日报：检查 worker / scheduler 日志与任务记录
```

- [ ] **Step 6: 运行联调文档校验命令**

Run: `rg -n "Holo|TODO|TBD" docs/deployment/integration-setup.md ; rg -n "/api/v1/jobs/daily-digest|/api/v1/digests/latest|/api/v1/dossiers|/api/v1/articles" docs/deployment/integration-setup.md ; rg -n "defaultAdminUsername|defaultAdminPassword" internal/service/admin_user_service.go`
Expected: 没有旧命名和占位词；接口路径与当前 OpenAPI/实现一致；默认管理员事实来源存在。

- [ ] **Step 7: Commit**

```bash
git add docs/deployment/integration-setup.md
git commit -m "docs: add integration setup guide"
```

### Task 5: 做全量一致性检查并收口

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\README.md`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\full-stack-ubuntu.md`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\fluxdigest-systemd.md`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs\docs\deployment\integration-setup.md`

- [ ] **Step 1: 扫描所有文档中的旧命名、占位词与错误环境变量**

```bash
rg -n "Holo|HOLO|TODO|TBD" README.md docs/deployment
test $? -eq 1
```

- [ ] **Step 2: 扫描关键环境变量、脚本名、接口路径是否全部命中**

```bash
rg -n "APP_PUBLISH_HALO_BASE_URL|APP_PUBLISH_HALO_TOKEN|APP_ADMIN_SESSION_SECRET|APP_SECRET_KEY" README.md docs/deployment
rg -n "deploy-systemd.sh|upgrade-systemd.sh|rollback-systemd.sh" README.md docs/deployment
rg -n "/api/v1/jobs/daily-digest|/api/v1/digests/latest|/api/v1/dossiers|/api/v1/articles" README.md docs/deployment
```

- [ ] **Step 3: 运行 git 差异格式检查**

Run: `git diff --check`
Expected: 无尾随空格、无损坏 patch、无冲突标记。

- [ ] **Step 4: 手工抽查链接与文件存在性**

Run: `Test-Path docs/deployment/full-stack-ubuntu.md ; Test-Path docs/deployment/fluxdigest-systemd.md ; Test-Path docs/deployment/integration-setup.md ; Get-Content README.md -TotalCount 120`
Expected: 3 篇文档都存在；README 顶部导航能正确指向它们。

- [ ] **Step 5: Commit**

```bash
git add README.md docs/deployment/full-stack-ubuntu.md docs/deployment/fluxdigest-systemd.md docs/deployment/integration-setup.md
git commit -m "docs: finalize deployment documentation set"
```

## Self-Review

- **Spec coverage:**
  - README 总入口：Task 1
  - Ubuntu 完整环境部署：Task 2
  - FluxDigest systemd 运维：Task 3
  - 首次联调：Task 4
  - 术语统一、脚本名/环境变量/接口路径一致性校验：Task 5
- **Placeholder scan:** 计划中未使用 `TODO` / `TBD` 作为实施占位；所有任务都指向实际文件与命令。
- **Type consistency:** 文档中统一使用 `Halo`、`Miniflux`、`FluxDigest WebUI`、`APP_PUBLISH_HALO_BASE_URL`、`APP_PUBLISH_HALO_TOKEN`、`APP_ADMIN_SESSION_SECRET`、`APP_SECRET_KEY`。
