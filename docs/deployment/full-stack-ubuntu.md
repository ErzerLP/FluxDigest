# FluxDigest 全栈部署指南（Ubuntu 22.04 / 24.04）
>
> 本文档遵循 2026-04-14 的部署设计说明与实现计划，面向首次在 Ubuntu x86_64 服务器上完成 Miniflux + Halo + FluxDigest 的完整上线。

## 适用范围
- 单节点 Ubuntu 22.04 / 24.04 x86_64 系统，非 Kubernetes/托管平台。
- 面向个人自用，希望按官方推荐手工部署 Miniflux、Halo 与 FluxDigest 的自托管用户。
- 假设拥有 sudo/root 权限，能够安装 Docker、Docker Compose、PostgreSQL、Redis 及 systemd 服务。

## 最终目标
1. 本地搭建 PostgreSQL、Redis、Miniflux、Halo。
2. 用 `deploy/scripts/deploy-systemd.sh` 把 FluxDigest API/Worker/Scheduler 注册到 systemd。
3. 在 FluxDigest WebUI 中完成 Miniflux/LLM/Halo 的初次配置，并通过 REST 接口触发一次日报验证链路。

## 端口规划
| 组件 | 默认端口 | 说明 |
| --- | --- | --- |
| FluxDigest WebUI / API | 18088 | `APP_HTTP_PORT`，也用于 healthz 与 OpenAPI。|
| PostgreSQL | 5432 | 与 `APP_DATABASE_DSN` 对应，建议仅监听内网。|
| Redis | 6379 | `APP_REDIS_ADDR` 默认 127.0.0.1:6379。|
| Miniflux | 28082 | Docker Compose 内部映射到 8080，供 FluxDigest 与浏览器访问。|
| Halo | 8090 | Halo 官方 Docker Compose 示例默认监听端口。|

修改端口需同时更新相应环境变量（如 `APP_MINIFLUX_BASE_URL`、`APP_PUBLISH_HALO_BASE_URL`）与 Docker Compose 配置。

## 基础准备
### 服务器前置要求
**做什么：** 更新系统、安装构建链与 Docker 工具，并保证能在服务器上构建 Go + Node.js。
**为什么：** 本教程默认直接在服务器上构建 FluxDigest，因此必须安装 Go/Node/npm；Docker 与 Docker Compose 用于运行 Miniflux 与 Halo。除非明确使用 `--skip-build`（见 `deploy/scripts/deploy-systemd.sh`）否则无法跳过构建。
**命令示例：**
```bash
sudo apt update
sudo apt install -y curl git ca-certificates gnupg lsb-release software-properties-common \
  apt-transport-https build-essential make
sudo apt install -y golang nodejs npm
```
然后按照官方文档安装 Docker 与 Docker Compose 插件（使用 `dpkg --print-architecture` 和 `lsb_release -cs` 获取架构与代号）：
```bash
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo "deb [arch=YOUR_ARCH signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu YOUR_CODENAME stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo systemctl enable --now docker
```
**成功后看到什么：** `docker version`、`docker compose version` 正常，`systemctl status docker` 显示 active，`go version`/`node -v`/`npm -v` 可用。

### PostgreSQL 与 Redis 准备
**做什么：** 使用 apt 安装 PostgreSQL 与 Redis，创建 `rss` 角色与数据库。
**为什么：** FluxDigest 依赖 PostgreSQL 与 Redis；Miniflux 可复用该 PostgreSQL，Halo 本文示例走官方 Compose 中的独立 `halodb` PostgreSQL 服务。
**命令示例：**
```bash
sudo apt install -y postgresql postgresql-contrib redis-server
sudo -u postgres psql -c "CREATE ROLE rss WITH LOGIN PASSWORD 'rss';"
sudo -u postgres psql -c "CREATE DATABASE rss OWNER rss;"
sudo systemctl enable --now postgresql redis-server
```
**成功后看到什么：** `psql -U rss -h 127.0.0.1 -d rss` 成功连接，`redis-cli ping` 返回 `PONG`。

## 部署 Miniflux
**做什么：** 在 `/opt/miniflux` 编写 Docker Compose，连接 PostgreSQL 并写入初始化环境变量。
**为什么：** FluxDigest 消费 Miniflux 抓取的文章流，Miniflux 只需 PostgreSQL 数据即可运行（参见 [Miniflux Docker 官方文档](https://miniflux.app/docs/docker.html)）。
**步骤示例：**
1. 创建目录并进入：
   ```bash
   sudo mkdir -p /opt/miniflux
   cd /opt/miniflux
   ```
2. 写入 `docker-compose.yml`，将 `DATABASE_URL`、管理员密码调整为真实值：
   ```yaml
   version: "3.9"
   services:
     miniflux:
       image: miniflux/miniflux:latest
       restart: unless-stopped
       ports:
         - 28082:8080
       environment:
         - DATABASE_URL=postgres://rss:rss@127.0.0.1:5432/rss?sslmode=disable
         - RUN_MIGRATIONS=1
         - CREATE_ADMIN=1
         - ADMIN_USERNAME=fluxdigest
         - ADMIN_PASSWORD=change-me
   ```
   > 替换点：`DATABASE_URL` 请调整为你的 PostgreSQL 主机/用户名/密码；`ADMIN_PASSWORD` 为初始密码，建议选强密码，Debian/Ubuntu 上可以用 `openssl rand -hex 16` 临时生成。
3. 启动服务：
   ```bash
   docker compose up -d
   ```
**成功后看到什么：** `docker compose ps` 中 `miniflux` 状态为 `Up` 且 `Errors` 列为空；访问 `http://<host>:28082/`，用 `fluxdigest`（或你在 Compose 中写的用户）登录后台并添加订阅源。

> 记得在 FluxDigest WebUI 中填写 Miniflux 的 `Base URL` 和 API Token，后续章节会说明。

## 部署 Halo
**做什么：** 在 `/opt/halo` 按照官方 Compose 示例包含 `halodb`（PostgreSQL）并启动 Halo，访问 `http://<host>:8090/` 完成初始化。
**为什么：** FluxDigest 的 `halo` 发布通道依赖 Halo WebIDE 与 PAT，推荐复用 [Halo Docker Compose 官方文档](https://docs.halo.run/getting-started/install/docker-compose) 的完整链路。
**步骤示例：**
1. 创建目录并进入：
   ```bash
   sudo mkdir -p /opt/halo
   cd /opt/halo
   ```
2. 写入 `docker-compose.yml`，替换 `<host>` 与密码：
   ```yaml
   version: "3.9"
   services:
     halodb:
       image: postgres:16
       restart: unless-stopped
       environment:
         - POSTGRES_USER=halo
         - POSTGRES_PASSWORD=halo-secret
         - POSTGRES_DB=halo
       volumes:
         - halodb-data:/var/lib/postgresql/data

     halo:
       image: registry.fit2cloud.com/halo/halo-pro:2.23
       restart: unless-stopped
       ports:
         - 8090:8090
       depends_on:
         - halodb
       command:
         - --spring.r2dbc.url=r2dbc:pool:postgresql://halodb/halo
         - --spring.r2dbc.username=halo
         - --spring.r2dbc.password=halo-secret
         - --spring.sql.init.platform=postgresql
         - --halo.external-url=http://<host>:8090/
       environment:
         - JAVA_OPTS=-Xmx1G

   volumes:
     halodb-data:
   ```
3. 启动 Halo 和数据库：
   ```bash
   docker compose up -d
   ```
4. 访问 `http://<host>:8090/`，按官方初始化向导设置管理员、站点、邮箱等，并在「个人中心 -> Personal Access Token」生成 PAT。
**成功后看到什么：** `docker compose ps` 中 `halodb` 和 `halo` 均为 `Up`，访问 `http://<host>:8090/admin` 能登录后台并看到已初始化的站点信息。

## 部署 FluxDigest
**做什么：** 拉取代码、复制 env 模板、编辑 `/etc/fluxdigest/fluxdigest.env`，运行系统脚本完成 systemd 部署。
**为什么：** `deploy/scripts/deploy-systemd.sh` 负责构建 Go/NPM、渲染 systemd 单元、部署 release 并启动 API/Worker/Scheduler。
**目录示例：**
```
/opt/fluxdigest/
  ├── releases/
  └── current -> releases/2026xxxxxxx
/etc/fluxdigest/fluxdigest.env
/etc/systemd/system/fluxdigest-api.service
```
**命令示例：**
```bash
cd /home/<your-user>
git clone https://github.com/ErzerLP/FluxDigest.git
cd FluxDigest
sudo mkdir -p /etc/fluxdigest
sudo cp deploy/systemd/fluxdigest.env.example /etc/fluxdigest/fluxdigest.env
sudo editor /etc/fluxdigest/fluxdigest.env
sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest \
  --env-file /etc/fluxdigest/fluxdigest.env
```
> 说明：脚本在发现目标 env 文件已存在时会保留原始内容，只会读取并替换已导出的变量。编辑时务必设置 PostgreSQL/Redis/Miniflux/Halo/LLM 等字段并生成安全密钥（`APP_ADMIN_SESSION_SECRET`、`APP_SECRET_KEY`、`APP_JOB_API_KEY`）。
**成功后看到什么：**
- `sudo systemctl status fluxdigest-api fluxdigest-worker fluxdigest-scheduler` 显示 active。
- `curl http://127.0.0.1:18088/healthz` 返回 `{"status":"ok"}`。
- `/etc/fluxdigest/fluxdigest.env` 中的变量与你编辑的值一致。

**说明：** 具体的 systemd 启停、升级、回滚、日志查看请参见 `docs/deployment/fluxdigest-systemd.md`。

## 首次登录与安全加固
**做什么：** 登录 FluxDigest WebUI 了解默认凭据，随后限制外网访问，确保环境变量中的 session/secret 采用强随机值。
**为什么：** `internal/service/admin_user_service.go` 确认首次部署会创建 `FluxDigest`/`FluxDigest` 种子管理员，并将 `must_change_password` 设为 true，但当前代码尚无 WebUI 或 API 可主动修改密码。也就是说，默认凭据属于临时状态，不能长期使用，必须通过限制暴露范围 + 强随机的 `APP_ADMIN_SESSION_SECRET`/`APP_SECRET_KEY` 来降低风险，后续管理员能力成熟后再更换凭据。
**步骤示例：**
1. 访问 `http://<host>:18088/`，使用 `FluxDigest` / `FluxDigest` 登录（若服务未就绪可查看 `journalctl -u fluxdigest-api`）。
2. 尽量在防火墙/NGINX 反向代理中限制 WebUI 访问来源，只在可信地址打开 18088 端口。
3. 在 `/etc/fluxdigest/fluxdigest.env` 中确保 `APP_ADMIN_SESSION_SECRET` 和 `APP_SECRET_KEY` 为随机 32 字节，`APP_JOB_API_KEY`、`APP_MINIFLUX_AUTH_TOKEN`、`APP_LLM_API_KEY`、`APP_PUBLISH_HALO_TOKEN` 等不使用示例值。
**成功后看到什么：** WebUI 提示默认管理员存在，`GET /api/v1/admin/status` 返回 `integrations` 相关数据，服务可以接受配置与触发请求。

## 填写配置并联调
**做什么：** 在 FluxDigest WebUI 的管理页面中录入 Miniflux/LLM/Publish 相关配置，并触发测试。
**为什么：** FluxDigest 通过 `APP_MINIFLUX_BASE_URL`、`APP_LLM_BASE_URL`、`APP_PUBLISH_HALO_BASE_URL` 读取这些配置，并通过 Admin API (`/api/v1/admin/configs/*`) 持久化。
**命令参考：**
- `PUT /api/v1/admin/configs/miniflux`：填写 `base_url`、`api_token`、`fetch_limit`、`lookback_hours` 等。
- `PUT /api/v1/admin/configs/llm`：填写 `base_url`、`model`、`timeout_ms`、`api_key`。
- `PUT /api/v1/admin/configs/publish`：channel 固定为 `halo`，填写 `halo_base_url` 与 `halo_token`。
- `POST /api/v1/admin/test/llm`：验证 LLM 可达。

在 WebUI 中点击「测试连接 / 保存」即可完成配置。详见 `docs/deployment/integration-setup.md` 中的配置与联调步骤。

## 验收检查清单
1. FluxDigest health：`curl http://127.0.0.1:18088/healthz` 返回 `{"status":"ok"}`。
2. Admin API：`curl http://127.0.0.1:18088/api/v1/admin/status`。
3. 触发日报：
   ```bash
   curl -X POST http://127.0.0.1:18088/api/v1/jobs/daily-digest \
     -H "X-API-Key: YOUR_APP_JOB_API_KEY"
   ```
   - 返回 `202 Accepted` 表示任务已入队。
4. 查询结果：
   - `GET /api/v1/digests/latest`
   - `GET /api/v1/articles`
   - `GET /api/v1/dossiers`
5. 检查日志：`systemctl status fluxdigest-*` 与 `journalctl -u fluxdigest-api -n 50 --no-pager`。
6. Halo 发布记录：在 FluxDigest WebUI 的发布模块中，确认 channel=halo 的 `publish_state` 为 `published` 或 `draft`。

## 常见问题
- **Miniflux 连接失败：** 确认 `APP_MINIFLUX_BASE_URL` 为 `http://127.0.0.1:28082`，并使用 Miniflux 后台生成的 API Token。
- **LLM 测试卡住：** 如果需要代理，请在 `/etc/fluxdigest/fluxdigest.env` 设置 `http_proxy` / `https_proxy`，并确认 LLM 服务的 TLS 证书可访问。
- **Halo 发布失败：** 检查 `APP_PUBLISH_CHANNEL=halo`、Halo PAT 有 `publish` 权限，`APP_PUBLISH_HALO_BASE_URL` 以 http/https 开头。
- **Job API 401：** `APP_JOB_API_KEY` 与 `X-API-Key` 必须一致，且在 env 中未被注释。
- **服务无法启动：** 先看 `journalctl -u fluxdigest-api`，如需 rollback 或 release 清理参见 `docs/deployment/fluxdigest-systemd.md`。
