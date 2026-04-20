# FluxDigest 全 Docker 一键安装器与 README 重构设计说明

- 日期：2026-04-14
- 设计主题：生产可用的一键安装器（Linux / Docker Compose）与后续标准 README 重写
- 工作区：`D:\Works\guaidongxi\RSS\.worktrees\full-stack-installer`
- 目标分支：`codex/full-stack-installer`
- 当前定位：个人自用、自托管、Linux 平台、以完整落地和低门槛安装为第一优先级

## 1. 背景

当前仓库已经有：

- FluxDigest 三进程（`rss-api` / `rss-worker` / `rss-scheduler`）
- React WebUI
- Miniflux / Halo 接入能力
- systemd 部署脚本
- 开发 smoke 用 Docker Compose

但对于首次使用者，仍然缺少一条真正闭环的“拿到仓库 -> 一条命令 -> 拉起完整环境 -> 拿到访问地址和账号密码 -> 进入 WebUI 继续配置 LLM”的落地路径。

同时，现有 README 已经证明不适合作为最终交付形态：

- 项目介绍不够像标准开源项目首页
- 快速开始不够直接
- 没有以一键安装脚本为主线
- 文档结构偏长、偏散、偏口语化

因此，本轮设计聚焦两个结果，且顺序固定：

1. **先完成完整的一键安装器**
2. **再基于真实安装器重写标准 README 和部署文档**

## 2. 已确认的约束与决策

以下内容已经在对话中明确，不再反复变更：

### 2.1 平台与安装方式

- 目标平台统一为 **Linux**
- 部署模型统一为 **Docker / Docker Compose**
- 安装器要求 **无人值守**
- Docker 与 Docker Compose **只做检测，不自动安装**
- 缺失时直接报错，并提示用户自行安装

### 2.2 安装目录与默认暴露方式

- 安装根目录固定为：`/opt/fluxdigest-stack`
- 默认不使用 systemd 作为主运行路径
- 默认不使用反向代理作为主运行路径
- 运行方式以 `docker compose up -d` 为准
- 服务通过宿主机端口直接访问

> 这里的“直接访问”指不强依赖 Nginx / Caddy。为了更稳妥，本文设计将 Web 面板端口对外开放，数据服务端口默认映射到宿主机，后续实现时优先采用更安全的绑定方式（例如数据库只绑定本机），但仍保持“宿主机端口直达”的使用模型。

### 2.3 凭据与初始化策略

- 安装过程中的凭据 **全部随机生成**
- 安装完成后必须明确输出：
  - PostgreSQL 连接信息
  - FluxDigest 管理后台用户名 / 密码
  - Miniflux 后台用户名 / 密码
  - Halo 管理员用户名 / 密码
  - 各服务访问入口
- **Miniflux / Halo 尽量自动接好**
- **LLM 不在安装器里配置**，保留给用户在 FluxDigest WebUI 手动填写

### 2.4 文档风格要求

- README 必须回到 **标准开源项目** 风格
- 必须提供真正可执行的 **Quick Start**
- 不再使用“这是干嘛的”这类口语标题
- README 应短、清晰、先讲价值、再给入口、再给一键安装命令

## 3. 本轮方案选择

## 3.1 候选方案

### 方案 A：独立的生产级 Stack Installer（选中）

在仓库内新增 `deploy/stack/`，维护完整的 Docker 安装器、模板和辅助脚本，目标是从源码仓库直接落地完整运行栈。

**优点：**

- 与现有 systemd 路径彻底解耦，不会把历史部署逻辑越堆越乱
- 可以天然承载“完整环境安装”“随机凭据”“安装摘要输出”“多 profile 安装”
- README 可以围绕单一入口重写，用户心智最简单
- 后续可以继续演进为升级、备份、卸载、诊断工具链

**缺点：**

- 需要单独维护一套 compose 模板与安装流程
- 需要补一部分应用侧的 bootstrap 配套能力

### 方案 B：继续扩展现有 systemd 脚本

把 Miniflux / Halo / PostgreSQL / Redis 的安装能力继续塞进现有 `deploy/scripts/*.sh`。

**缺点：**

- systemd 脚本原本只负责 FluxDigest 三进程，边界已经很清楚
- 把整栈逻辑塞进去会让已有部署路径和新路径互相污染
- 与“全部使用 Docker”这一已确认决策冲突

### 方案 C：只写文档，不提供真正的一键安装器

**缺点：**

- 与用户已明确要求的“一键安装脚本”直接冲突
- 不能解决 README 缺少真正快速开始的问题

## 3.2 结论

采用 **方案 A：独立的生产级 Stack Installer**。

## 4. 目标范围与非目标

### 4.1 本轮目标范围

本轮最终要交付的能力包括：

1. 基于 Docker Compose 的 FluxDigest 安装器
2. 支持多种安装 profile：
   - `full`：FluxDigest + Miniflux + Halo + PostgreSQL + Redis
   - `fluxdigest-miniflux`：FluxDigest + Miniflux + PostgreSQL + Redis
   - `fluxdigest-halo`：FluxDigest + Halo + PostgreSQL + Redis
   - `fluxdigest-only`：FluxDigest + PostgreSQL + Redis
3. 自动生成随机凭据并落盘摘要
4. 自动初始化数据库与基础管理员账户
5. 自动把 Miniflux / Halo 的地址和默认接线写入 FluxDigest 运行配置
6. LLM 配置在安装后由用户通过 WebUI 手工填写
7. 增加 FluxDigest 管理员 bootstrap 环境变量支持
8. 在安装器完成后，重写 README 与配套部署文档

### 4.2 明确非目标

以下内容不在本轮主目标范围内：

- 自动安装 Docker / Docker Compose
- Kubernetes / Helm 部署
- 把现有 smoke compose 直接升级成生产安装器
- 把 systemd 路径继续作为默认部署方案
- 自动为用户配置 Docker Daemon 代理
- 自动申请域名、TLS 证书、反向代理
- 自动完成用户自己的 LLM 平台接入

## 5. 总体架构

### 5.1 完整栈服务组成

默认完整栈包含以下服务：

- `postgres`
- `redis`
- `miniflux`
- `halo`
- `fluxdigest-api`
- `fluxdigest-worker`
- `fluxdigest-scheduler`

其中：

- `postgres` 为共享数据库实例，承载 `fluxdigest` / `miniflux` / `halo` 三个数据库
- `redis` 由 FluxDigest 使用；Halo 暂不默认启用 Redis Session 能力
- `miniflux` 负责 RSS 聚合
- `halo` 负责博客发布
- `fluxdigest-*` 为应用三进程

### 5.2 默认端口规划

默认端口采用以下规划：

| 服务 | 默认端口 | 说明 |
| --- | --- | --- |
| FluxDigest WebUI / API | `18088` | 用户主要入口 |
| Miniflux | `28082` | RSS 订阅管理后台 |
| Halo | `28090` | 博客后台与站点 |
| PostgreSQL | `35432` | 数据库连接端口 |
| Redis | `36379` | Redis 调试端口 |

### 5.3 目录布局

仓库内新增：

```text
deploy/
└─ stack/
   ├─ install.sh
   ├─ docker-compose.yml.tpl
   ├─ stack.env.tpl
   ├─ initdb/
   │  └─ 01-init-app-dbs.sql.tpl
   └─ scripts/
      ├─ common.sh
      ├─ render.sh
      ├─ healthcheck.sh
      ├─ bootstrap_miniflux.sh
      └─ bootstrap_halo.sh
```

目标机安装目录固定为：

```text
/opt/fluxdigest-stack/
├─ .env
├─ docker-compose.yml
├─ install-summary.txt
├─ initdb/
├─ data/
│  ├─ postgres/
│  ├─ redis/
│  ├─ halo/
│  ├─ miniflux/
│  └─ fluxdigest/
└─ logs/
```

## 6. 安装器行为设计

### 6.1 执行入口

安装命令以仓库源码为入口：

```bash
sudo bash deploy/stack/install.sh
```

安装器默认 profile 为 `full`，并支持：

```bash
sudo bash deploy/stack/install.sh --profile full
sudo bash deploy/stack/install.sh --profile fluxdigest-miniflux
sudo bash deploy/stack/install.sh --profile fluxdigest-halo
sudo bash deploy/stack/install.sh --profile fluxdigest-only
```

附加选项保留最小必要集合：

- `--profile <name>`
- `--stack-dir <path>`，默认 `/opt/fluxdigest-stack`
- `--force`，覆盖已有目标目录中的生成文件

不在第一版引入过多 flags，避免 README 和脚本一起复杂化。

### 6.2 预检查

安装器启动后依次检查：

1. 当前系统是否为 Linux
2. 当前用户是否具备写入目标目录和调用 Docker 的能力
3. `docker` 是否存在
4. `docker compose` 是否存在
5. `openssl`、`sed`、`awk`、`curl` 等基础命令是否存在
6. 当前仓库是否包含：
   - `deployments/docker/api.Dockerfile`
   - `deployments/docker/worker.Dockerfile`
   - `deployments/docker/scheduler.Dockerfile`

任意一项缺失都直接失败，并打印明确错误。

### 6.3 凭据生成

安装器会生成并保存以下随机值：

- PostgreSQL：
  - `POSTGRES_ROOT_PASSWORD`
  - `FLUXDIGEST_DB_USER` / `FLUXDIGEST_DB_PASSWORD`
  - `MINIFLUX_DB_USER` / `MINIFLUX_DB_PASSWORD`
  - `HALO_DB_USER` / `HALO_DB_PASSWORD`
- FluxDigest：
  - `FLUXDIGEST_ADMIN_USERNAME`
  - `FLUXDIGEST_ADMIN_PASSWORD`
  - `APP_ADMIN_SESSION_SECRET`
  - `APP_SECRET_KEY`
  - `APP_JOB_API_KEY`
- Miniflux：
  - `MINIFLUX_ADMIN_USERNAME`
  - `MINIFLUX_ADMIN_PASSWORD`
- Halo：
  - `HALO_ADMIN_USERNAME`
  - `HALO_ADMIN_PASSWORD`
  - `HALO_ADMIN_EMAIL`
  - `HALO_PAT_NAME`

LLM 相关字段在 `.env` 中保留为空或注释引导，不在安装时生成假的默认值。

### 6.4 数据库初始化

共享 PostgreSQL 容器通过 `initdb` 模板创建：

- `fluxdigest` 数据库与专用用户
- `miniflux` 数据库与专用用户
- `halo` 数据库与专用用户

这样避免所有服务共用同一数据库用户，边界更清晰，也方便安装摘要直接给出每个系统的连接信息。

### 6.5 镜像构建

安装器会在仓库根目录执行本地镜像构建：

- `fluxdigest-api`
- `fluxdigest-worker`
- `fluxdigest-scheduler`

镜像构建依赖当前仓库源码，不要求预先存在远端镜像仓库。

### 6.6 启动顺序

安装器采用分阶段启动，避免一次性全部起来后难以定位错误：

#### 阶段 1：基础服务

按 profile 启动：

- `postgres`
- `redis`
- `miniflux`（若 profile 包含）
- `halo`（若 profile 包含）

并等待健康检查通过。

#### 阶段 2：外部组件 bootstrap

- Miniflux：
  - 通过官方 Docker 环境变量 `DATABASE_URL`、`RUN_MIGRATIONS=1`、`CREATE_ADMIN=1`、`ADMIN_USERNAME`、`ADMIN_PASSWORD` 初始化
  - 安装器再通过 Miniflux API 验证登录令牌与基础连通性
- Halo：
  - 官方当前文档展示了 Docker Compose + PostgreSQL 的启动参数与首次初始化页面
  - **推断设计**：为了满足“自动接好 Halo”的要求，安装器将优先使用 Halo 的 superadmin initializer 启动参数完成管理员 bootstrap，然后通过管理接口创建供 FluxDigest 使用的 PAT
  - 如果现场验证发现当前 Halo 版本不再支持该 bootstrap 方式，本设计不做静默降级，而是回到设计阶段重新确定初始化策略

#### 阶段 3：FluxDigest 三服务

启动：

- `fluxdigest-api`
- `fluxdigest-worker`
- `fluxdigest-scheduler`

并执行应用健康检查。

### 6.7 FluxDigest 运行时接线

安装器渲染的 `.env` 需要直接把以下运行时配置接入应用：

- `APP_DATABASE_DSN`
- `APP_REDIS_ADDR`
- `APP_HTTP_PORT`
- `APP_JOB_API_KEY`
- `APP_ADMIN_SESSION_SECRET`
- `APP_SECRET_KEY`
- `APP_ADMIN_BOOTSTRAP_USERNAME`
- `APP_ADMIN_BOOTSTRAP_PASSWORD`
- `APP_MINIFLUX_BASE_URL`（profile 包含 Miniflux 时自动填入）
- `APP_MINIFLUX_AUTH_TOKEN`（由安装器 bootstrap 后填入）
- `APP_PUBLISH_CHANNEL`
- `APP_PUBLISH_HALO_BASE_URL`（profile 包含 Halo 时自动填入）
- `APP_PUBLISH_HALO_TOKEN`（由安装器创建 PAT 后填入）
- `APP_LLM_BASE_URL`
- `APP_LLM_API_KEY`
- `APP_LLM_MODEL`
- `APP_LLM_TIMEOUT_MS`

其中：

- 若 profile 不包含 Miniflux，则 `APP_MINIFLUX_*` 留空，用户后续在 WebUI 配置外部 Miniflux
- 若 profile 不包含 Halo，则发布器默认退回 `markdown_export`
- 若 profile 包含 Halo，则发布器默认配置为 `halo`
- LLM 默认留空，不阻塞安装完成和 WebUI 登录

### 6.8 安装摘要输出

安装成功后，安装器必须同时：

1. 在终端打印摘要
2. 在 `/opt/fluxdigest-stack/install-summary.txt` 落盘摘要

摘要至少包含：

- 当前安装 profile
- 各服务访问地址
- FluxDigest / Miniflux / Halo 的管理员用户名与密码
- PostgreSQL 三套数据库用户名 / 密码 / 数据库名 / 端口
- Redis 地址
- LLM 尚未配置的提醒
- 常用运维命令：
  - `docker compose ps`
  - `docker compose logs -f fluxdigest-api`
  - `docker compose restart`

## 7. 应用侧必要配套改动

### 7.1 FluxDigest 管理员 bootstrap 环境变量

当前 `internal/service/admin_user_service.go` 写死了默认管理员：

- 用户名：`FluxDigest`
- 密码：`FluxDigest`

这与“随机凭据安装”直接冲突，因此必须改为：

- `APP_ADMIN_BOOTSTRAP_USERNAME`
- `APP_ADMIN_BOOTSTRAP_PASSWORD`

行为定义如下：

1. 若数据库中已经存在管理员用户，则不重复创建
2. 若数据库中没有管理员用户，则优先读取 `APP_ADMIN_BOOTSTRAP_USERNAME` / `APP_ADMIN_BOOTSTRAP_PASSWORD`
3. 若环境变量未设置，则回退为 `FluxDigest / FluxDigest`，保持旧行为兼容
4. 首次创建的管理员仍标记为 `MustChangePassword=true`

### 7.2 配套代码修改范围

本轮安装器实现预计至少会触及：

- `internal/service/admin_user_service.go`
- `internal/service/admin_user_service_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/rss-api/main.go`
- `cmd/rss-api/main_test.go`
- `deploy/stack/*`

如果实际 wiring 需要，也允许新增很小范围的辅助类型，但不做与安装器无关的结构性重构。

## 8. 配置与安全策略

### 8.1 LLM 配置策略

安装器不会替用户猜测任何真实 LLM 平台，也不会填演示值。

安装完成后，用户通过 FluxDigest WebUI 填写：

- Base URL
- API Key
- Model
- Timeout
- Prompt 模板

这样可以保持安装器对外部 AI 平台零耦合。

### 8.2 代理策略

本轮不负责配置 Docker Daemon 代理，也不修改宿主机全局代理。

但安装器会：

- 检测当前 shell 的 `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY`
- 若用户已显式导出这些环境变量，则把它们透传给需要访问外部网络的 FluxDigest 容器
- 在安装失败提示中明确区分：
  - Docker 拉镜像失败
  - 应用访问外部 LLM 失败

### 8.3 端口暴露策略

默认保持“宿主机端口直达”模型，但优先采用更安全的默认值：

- 面板类服务（FluxDigest / Miniflux / Halo）可直接通过宿主机端口访问
- 数据类服务（PostgreSQL / Redis）默认暴露宿主机端口，便于调试和自用，但文档中会明确提醒只应在受控网络环境使用

## 9. README 与部署文档的后续重构方向

README 重构必须发生在安装器完成并验证之后，避免文档再次超前于实现。

### 9.1 README 目标

新的 `README.md` 应满足：

- 用户打开仓库首页后，10 秒内知道项目价值和主工作流
- 首页能直接看到 Quick Start
- 首页能直接看到安装后会得到哪些访问入口
- 首页明确告知：LLM 需要安装后在 WebUI 中配置
- 首页不再承担过长的逐命令教程

### 9.2 README 推荐结构

新的 README 采用标准开源项目结构：

1. 项目名与一句话简介
2. Overview
3. Features
4. Architecture / Workflow
5. Quick Start
6. Configuration
7. Publishing Targets
8. API
9. Deployment Docs
10. Development
11. License

### 9.3 Quick Start 设计

README 中的 Quick Start 必须围绕真正的一键安装器，例如：

```bash
git clone https://github.com/ErzerLP/FluxDigest.git
cd FluxDigest
sudo bash deploy/stack/install.sh --profile full
```

Quick Start 下方直接说明：

- 默认安装目录
- 默认访问地址
- 安装完成后去哪里看生成的账号密码
- 下一步去 FluxDigest WebUI 配置 LLM

### 9.4 配套文档最小集合

README 重构后，详细文档收敛为尽量少的几篇：

- `docs/deployment/docker-stack.md`：完整安装 / 升级 / 重装 / 卸载
- `docs/deployment/runtime-configuration.md`：LLM、Miniflux、Halo、Prompt、发布策略配置
- `docs/api/open-api-guide.md`：继续作为开放接口总览

不再把 README 写成冗长“全内容索引页”。

## 10. 验证标准

本轮设计对应的最终实现，必须满足以下验收标准：

### 10.1 安装器验收

1. 在一台已安装 Docker / Docker Compose 的 Linux 机器上，可通过一条命令完成安装
2. `full` profile 能成功拉起 FluxDigest、Miniflux、Halo、PostgreSQL、Redis
3. `fluxdigest-miniflux` / `fluxdigest-halo` / `fluxdigest-only` profile 能按预期裁剪服务
4. 安装结束后，用户可以直接拿到所有访问入口和账号密码
5. FluxDigest WebUI 可登录，但 LLM 配置为空，等待用户填写
6. Miniflux / Halo 若被纳入 profile，其地址已自动写入 FluxDigest 默认配置
7. 生成的 `.env`、`docker-compose.yml`、`install-summary.txt` 都位于 `/opt/fluxdigest-stack`

### 10.2 文档验收

1. README 首页出现真正 Quick Start
2. README 风格符合标准开源项目习惯，不再口语化
3. 文档内容与安装器真实行为一致
4. 文档中明确写出默认目录、访问地址、账号密码查看位置、LLM 后配置步骤

## 11. 实现顺序

本轮实现顺序固定如下：

1. 先实现 `APP_ADMIN_BOOTSTRAP_*` 配套改动
2. 再实现 `deploy/stack/` 安装器、模板、bootstrap 脚本
3. 在真实测试机上完成完整安装验证
4. 基于真实安装结果重写 README 与部署文档
5. 再做文档校对与最终交付

## 12. 需要特别坚持的原则

- 不为了兼容旧 README 而牺牲安装器入口清晰度
- 不为了偷快而把 smoke compose 直接当生产安装器
- 不为了“看起来自动化”而写一堆静默 fallback
- 不把尚未验证的能力写成 README 里的既成事实
- 安装失败时宁可明确报错，也不交付半成功状态

## 13. 设计结论

这次工作的核心不是“再补几段部署说明”，而是给 FluxDigest 建立一条真正成立的产品化安装路径：

- **安装层**：`deploy/stack/install.sh` 成为唯一明确的一键入口
- **运行层**：Docker Compose 承担标准 Linux 自托管方案
- **配置层**：Miniflux / Halo 自动接线，LLM 安装后在 WebUI 中手动完成
- **文档层**：README 退回标准开源首页职责，只保留清晰价值说明、Quick Start 和文档导航

该设计完成后，FluxDigest 的首次落地路径会从“自己拼组件”收敛为“克隆仓库 -> 运行脚本 -> 拿到入口和密码 -> 登录 WebUI 配置 LLM”。
