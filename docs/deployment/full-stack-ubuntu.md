# FluxDigest 全栈部署指南（Linux / Docker）

本文档面向第一次在 Linux 主机上部署 FluxDigest 的用户。当前推荐路径只有一条：**克隆仓库后直接运行根目录 `install.sh`**。

## 适用范围

- 单机或小型自托管 Linux 环境
- 通过 Docker / Docker Compose 运行 FluxDigest、Miniflux、Halo
- 个人自用、平台优先、希望尽量减少手工步骤的部署方式

## 部署前准备

请先确认以下命令已经存在：

- `docker`
- `docker compose`
- `git`
- `curl`
- `whiptail`

推荐先手动检查：

```bash
docker version
docker compose version
git --version
curl --version
whiptail --version
```

> 安装脚本会自动检测这些依赖；如果缺失，只会报错提示，不会自动安装。

## 运行安装器

```bash
git clone https://github.com/ErzerLP/FluxDigest.git
cd FluxDigest
bash install.sh
```

安装器会进入交互式菜单，全部选择类步骤都通过 `whiptail` 完成：

- 方向键上下选择
- Tab 切换按钮
- Enter 确认

## 可选部署组合

安装器目前支持以下组合：

- `FluxDigest + Miniflux + Halo`
- `FluxDigest + Miniflux`
- `FluxDigest + Halo`
- `FluxDigest only`

如果你只想尽快体验，直接选择 **快速安装（推荐）** 即可。

## 安装完成后会生成什么

安装成功后，脚本会生成并展示安装摘要，默认位于：

```text
/opt/fluxdigest-stack/install-summary.txt
```

摘要中会列出：

- FluxDigest / Miniflux / Halo 的访问地址
- 默认管理员账号密码
- PostgreSQL 与 Redis 信息
- `.env`、`docker-compose.yml`、`install-summary.txt` 路径
- 当前 release 信息

`install-summary.txt` 默认保留敏感权限。如果你只是想看当前 release / 访问地址 / 镜像状态，可直接执行：

```bash
bash install.sh --action status --stack-dir /opt/fluxdigest-stack
```

如果需要查看完整账号密码，再使用：

```bash
sudo cat /opt/fluxdigest-stack/install-summary.txt
```

安装目录中还会生成：

```text
/opt/fluxdigest-stack/
├─ .env
├─ docker-compose.yml
├─ install-summary.txt
├─ releases/
└─ data/
```

## 安装后第一件事

1. 打开安装摘要，确认面板地址和默认账号密码
2. 登录 FluxDigest WebUI
3. 在 WebUI 中配置 LLM
4. 打开 Miniflux 后台添加 RSS 订阅源
5. 如启用了 Halo，补充 Halo 发布令牌

## 升级、回滚、状态查看

这些动作仍然通过同一个入口完成：

```bash
bash install.sh --action upgrade --stack-dir /opt/fluxdigest-stack
bash install.sh --action rollback --stack-dir /opt/fluxdigest-stack --release-id 20260415070001
bash install.sh --action status --stack-dir /opt/fluxdigest-stack
```

更详细的参数说明见：[`installer-reference.md`](./installer-reference.md)

## 常见提示

- 如果是首次安装，优先使用 `bash install.sh` 进入交互菜单，不必记复杂参数。
- 如果 LLM 服务需要代理，请在安装后的 `.env` 或运行环境中配置 `http_proxy` / `https_proxy`。
- RSS 订阅源始终在 Miniflux 后台维护，FluxDigest 只消费 Miniflux 已抓取的文章。
