# FluxDigest

> 把 Miniflux 里的 RSS 文章自动翻译、分析、摘要，生成“每日汇总日报”，并发布到 Halo 或通过 API 提供给其他系统。

## 这是干嘛的

FluxDigest 是一个面向个人自托管场景的 RSS 智能处理平台：

- **Miniflux** 负责 RSS 订阅与抓取
- **FluxDigest** 负责 AI 翻译、分析、摘要、日报聚合
- **输出** 可以进入 Halo、Markdown 导出，或通过开放 API 被其他系统消费

如果你已经在用 Miniflux，但不想每天自己读完所有订阅，FluxDigest 就是把“订阅流”变成“可读日报和单篇解读”的那层自动化。

## 核心能力

- 单篇文章翻译、分析、摘要与结构化 dossier 产出
- 自动聚合生成“每日汇总日报”
- WebUI 在线配置 LLM、Miniflux、发布通道与提示词
- 发布到 Halo，并通过 API 对外提供处理结果

## 快速开始

### 1) 完整环境快速开始（推荐）

适合第一次从 0 部署完整环境的用户。
这条路径会带你完成 **PostgreSQL + Redis + Miniflux + Halo + FluxDigest** 的完整搭建。

- 入口文档：[`docs/deployment/full-stack-ubuntu.md`](docs/deployment/full-stack-ubuntu.md)
- 适用系统：Ubuntu 22.04 / 24.04 x86_64
- 这是当前最快的完整环境落地路径

### 2) FluxDigest 本体一键部署（已有依赖时）

如果你已经准备好 PostgreSQL、Redis、Miniflux 和 Halo，可以直接使用现有 systemd 部署脚本：

```bash
sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest
```

这条脚本会负责：

- 构建 Go 二进制与 WebUI
- 安装 release 到目标目录
- 切换 `current` 软链
- 安装 / 重启 `rss-api`、`rss-worker`、`rss-scheduler`
- 执行健康检查

进一步的升级与回滚说明见：[`docs/deployment/fluxdigest-systemd.md`](docs/deployment/fluxdigest-systemd.md)

### 3) 本地 smoke / 开发验证

如果你只是想先验证主流程或本地开发调试，可以运行：

```powershell
./scripts/smoke-compose.ps1
```

它适合开发验证，不是生产部署入口。

## 部署完成后先看这里

| 入口 | 默认地址 | 用途 |
| --- | --- | --- |
| FluxDigest WebUI / API | `http://<host>:18088/` | 配置 LLM、Miniflux、发布通道、提示词，查看任务状态 |
| Miniflux 后台 | `http://<host>:28082/` | 添加和管理 RSS 订阅源 |
| Halo 后台 | `http://<host>:8090/` | 查看和管理最终发布内容 |
| 开放接口文档 | [`docs/api/open-api-guide.md`](docs/api/open-api-guide.md) | 查看人工可读接口总览 |
| OpenAPI | [`api/openapi/openapi.yaml`](api/openapi/openapi.yaml) | 查看机器可读接口契约 |

## 最常见的使用流程

1. 在 **Miniflux 后台** 添加 RSS 订阅源
2. 在 **FluxDigest WebUI** 配置 LLM、Miniflux、Halo / 发布通道
3. 手动触发或等待定时日报生成
4. 在 **Halo** 或 **开放 API** 中查看单篇解读和每日汇总结果

> 注意：RSS 订阅源管理始终在 Miniflux 中完成；FluxDigest 不负责订阅管理，只处理 Miniflux 已抓取的内容。

## 文档导航

- 完整环境部署：[`docs/deployment/full-stack-ubuntu.md`](docs/deployment/full-stack-ubuntu.md)
- systemd 部署 / 升级 / 回滚：[`docs/deployment/fluxdigest-systemd.md`](docs/deployment/fluxdigest-systemd.md)
- 联调与配置：[`docs/deployment/integration-setup.md`](docs/deployment/integration-setup.md)
- 开放接口总览：[`docs/api/open-api-guide.md`](docs/api/open-api-guide.md)
- OpenAPI：[`api/openapi/openapi.yaml`](api/openapi/openapi.yaml)

## 开发者入口

### 本地开发

```bash
make run-api
make run-worker
make run-scheduler
```

前端开发：

```bash
npm --prefix web ci
npm --prefix web run dev
```

### 测试命令

```bash
go test ./...
npm --prefix web test -- --run
```

### 运行结构

- `rss-api`：API、WebUI 静态资源托管、管理接口
- `rss-worker`：文章处理、日报生成、发布执行
- `rss-scheduler`：定时触发任务

### 目录结构（简版）

```text
.
├─ api/openapi/           # OpenAPI 契约
├─ cmd/                   # 三个进程入口
├─ configs/               # 示例配置与默认 prompts
├─ deploy/                # systemd 部署脚本与模板
├─ docs/                  # 部署、接口与其他说明文档
├─ internal/              # 应用实现
└─ web/                   # React WebUI
```

### 扩展参考

- 发布渠道扩展：`internal/adapter/publisher/`
- LLM 接入：`internal/adapter/llm/`
- 运行时 profile：`internal/domain/profile/`
