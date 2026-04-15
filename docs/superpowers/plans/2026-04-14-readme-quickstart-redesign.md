# README Quickstart Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `README.md` 重构成“项目入口页 + 快速开始 + 文档导航”，让首次用户能快速看懂项目并找到可执行入口，同时保留开发者导航。

**Architecture:** 以 `README.md` 为主改文件，重写首页前半部分的叙事顺序：一句话介绍、核心能力、三个快速开始入口、部署后入口、使用流程、文档导航，再把开发者说明收敛到后半部分。实现时以现有脚本和文档为事实来源，不虚构不存在的完整环境一键安装器。必要时只做少量文档命名/导航一致性同步，不修改部署脚本逻辑。

**Tech Stack:** Markdown、PowerShell、Python、Git

---

## File Map

- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md` — 重写首页信息架构与快速开始入口
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\deploy\scripts\deploy-systemd.sh` — FluxDigest 本体 systemd 部署脚本入口
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\scripts\smoke-compose.ps1` — 本地 smoke 入口
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\deployment\full-stack-ubuntu.md` — 完整环境部署入口
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\deployment\fluxdigest-systemd.md` — systemd 部署文档
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\deployment\integration-setup.md` — 联调与使用流程文档
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\api\open-api-guide.md` — 开放接口总览

### Task 1: 重写 README 首页与快速开始入口

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md`

- [ ] **Step 1: 读取现有 README，确认待替换的旧结构特征**

Run:

```powershell
@'
from pathlib import Path
path = Path(r'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md')
text = path.read_text(encoding='utf-8')
required = [
    '## 1) 项目介绍',
    '## 4) 快速开始（本地开发）',
    '## 6) 部署教程',
    '## 7) WebUI 用途说明',
]
missing = [item for item in required if item not in text]
if missing:
    raise SystemExit('README structure changed unexpectedly: ' + ', '.join(missing))
print('PASS: existing README structure confirmed')
'@ | python -
```

Expected: 输出 `PASS: existing README structure confirmed`

- [ ] **Step 2: 用新的首页结构整体替换 README**

```powershell
@'
# FluxDigest

FluxDigest 会从 Miniflux 拉取 RSS 文章，调用 AI 做翻译、分析和摘要，自动生成“每日汇总日报”，并可发布到 Halo 或通过 API 提供给其他系统。

## 这是什么

FluxDigest 是一个面向个人自托管场景的 RSS 智能处理平台：

- **输入**：Miniflux 已抓取的 RSS 文章
- **处理**：AI 翻译、分析、摘要、日报聚合
- **输出**：Halo 博客内容、Markdown 导出、开放 API

它适合已经在使用 Miniflux、希望把订阅内容自动整理成中文解读和每日日报的用户。

## 核心能力

- 单篇文章翻译、分析、摘要与结构化 dossier 产出
- 自动聚合生成“每日汇总日报”
- WebUI 在线配置 LLM、Miniflux、发布通道与提示词
- 发布到 Halo，并通过 API 对外提供处理结果

## 快速开始

### 1) 完整环境快速开始（推荐）

适合第一次部署完整环境的用户。
这条路径会带你完成 **PostgreSQL + Redis + Miniflux + Halo + FluxDigest** 的完整搭建：

- 入口文档：[`docs/deployment/full-stack-ubuntu.md`](docs/deployment/full-stack-ubuntu.md)
- 适用系统：Ubuntu 22.04 / 24.04 x86_64
- 部署完成后，你可以直接访问 FluxDigest WebUI、Miniflux 后台与 Halo 后台

### 2) 只部署 FluxDigest（已有依赖时）

如果你已经准备好 PostgreSQL、Redis、Miniflux 和 Halo，可以直接使用现有 systemd 部署脚本：

```bash
sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest
```

这条脚本会负责：

- 构建 Go 二进制与 WebUI
- 安装 release 到目标目录
- 更新 `current` 软链
- 安装 / 重启 `rss-api`、`rss-worker`、`rss-scheduler`
- 执行健康检查

进一步的 systemd、升级与回滚说明见：[`docs/deployment/fluxdigest-systemd.md`](docs/deployment/fluxdigest-systemd.md)

### 3) 本地 smoke / 开发验证

如果你只是想先验证主流程或开发调试，可以运行本地 smoke 脚本：

```powershell
./scripts/smoke-compose.ps1
```

它适合本地开发验证，不是生产部署入口。

## 部署完成后你会用到的入口

- **FluxDigest WebUI**：用于配置 LLM、Miniflux、发布通道、提示词与查看任务状态
- **Miniflux 后台**：用于添加和管理 RSS 订阅源
- **Halo 后台**：用于查看和管理最终发布内容
- **开放接口文档**：[`docs/api/open-api-guide.md`](docs/api/open-api-guide.md)
- **OpenAPI**：[`api/openapi/openapi.yaml`](api/openapi/openapi.yaml)

## 最常见的使用流程

1. 打开 **Miniflux 后台** 添加 RSS 订阅源
2. 打开 **FluxDigest WebUI** 配置 LLM、Miniflux、Halo / 发布通道
3. 手动触发或等待定时日报生成
4. 在 **Halo** 或 **开放 API** 中查看单篇解读和每日汇总结果

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
'@ | Set-Content -Path 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md' -Encoding UTF8
```

- [ ] **Step 3: 校验 README 新结构与关键入口是否存在**

Run:

```powershell
@'
from pathlib import Path
root = Path(r'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign')
text = (root / 'README.md').read_text(encoding='utf-8')
required = [
    'FluxDigest 会从 Miniflux 拉取 RSS 文章',
    '## 快速开始',
    '### 1) 完整环境快速开始（推荐）',
    '### 2) 只部署 FluxDigest（已有依赖时）',
    'sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest',
    './scripts/smoke-compose.ps1',
    '## 部署完成后你会用到的入口',
    '## 最常见的使用流程',
    '## 文档导航',
    'docs/deployment/full-stack-ubuntu.md',
    'docs/deployment/fluxdigest-systemd.md',
    'docs/deployment/integration-setup.md',
    'docs/api/open-api-guide.md',
    'api/openapi/openapi.yaml',
]
missing = [item for item in required if item not in text]
if missing:
    raise SystemExit('README missing expected items: ' + ', '.join(missing))
print('PASS: README quickstart structure verified')
'@ | python -
```

Expected: 输出 `PASS: README quickstart structure verified`

- [ ] **Step 4: 提交 README 重构**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' add 'README.md'
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' commit -m "docs: redesign readme quickstart"
```

Expected: 生成只包含 `README.md` 的提交

### Task 2: 核对文档入口与命名一致性

**Files:**
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\deployment\full-stack-ubuntu.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\deployment\fluxdigest-systemd.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\deployment\integration-setup.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\docs\api\open-api-guide.md`

- [ ] **Step 1: 校验 README 中引用的文档路径全部存在**

Run:

```powershell
@'
from pathlib import Path
root = Path(r'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign')
paths = [
    'docs/deployment/full-stack-ubuntu.md',
    'docs/deployment/fluxdigest-systemd.md',
    'docs/deployment/integration-setup.md',
    'docs/api/open-api-guide.md',
    'api/openapi/openapi.yaml',
]
missing = [item for item in paths if not (root / item).exists()]
if missing:
    raise SystemExit('Missing linked files: ' + ', '.join(missing))
print('PASS: README linked files exist')
'@ | python -
```

Expected: 输出 `PASS: README linked files exist`

- [ ] **Step 2: 扫描 README 中的旧信息，确认已清理**

Run:

```powershell
@'
from pathlib import Path
text = Path(r'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md').read_text(encoding='utf-8')
forbidden = ['holo', 'article-reprocess', '占位壳层', '关键配置项概览']
found = [item for item in forbidden if item in text.lower() or item in text]
if found:
    raise SystemExit('README still contains stale items: ' + ', '.join(found))
print('PASS: stale README phrases removed')
'@ | python -
```

Expected: 输出 `PASS: stale README phrases removed`

- [ ] **Step 3: 对最终 README 做 diff 级自检**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' diff --check origin/master...HEAD
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' diff --stat origin/master...HEAD
```

Expected:
- `git diff --check` 无输出
- `git diff --stat` 主要显示 `README.md` 被重写

- [ ] **Step 4: 提交任何必要的 README 文案收尾**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' status --short
```

Expected: 若无额外改动则输出为空；若因 Task 2 产生 README 微调，则提交信息使用：

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' add 'README.md'
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' commit -m "docs: polish readme navigation"
```

### Task 3: 完成交付流程验证

**Files:**
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\README.md`

- [ ] **Step 1: 运行项目基线测试，确认 README 改动未污染工作区**

Run:

```powershell
go test ./...
npm test -- --run
```

Workdirs:
- Go：`D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign`
- Web：`D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign\web`

Expected:
- Go 测试通过
- Web 测试通过

- [ ] **Step 2: 推送分支并在测试服务器拉取核对**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' push origin codex/readme-quickstart-redesign
```

Then on test server repository `/home/hjx/FluxDigest-admin-config-pages-live`:

```bash
git fetch git@github.com:ErzerLP/FluxDigest.git refs/heads/codex/readme-quickstart-redesign:refs/remotes/origin/codex/readme-quickstart-redesign
git checkout -B codex/readme-quickstart-redesign origin/codex/readme-quickstart-redesign
git rev-parse HEAD
sed -n '1,120p' README.md
```

Expected:
- 远端分支存在
- 测试服务器 README 能看到新的一句话介绍、快速开始与文档导航结构

- [ ] **Step 3: 合并回 master**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' fetch origin master
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' merge-base --is-ancestor origin/master codex/readme-quickstart-redesign
git -C 'D:\Works\guaidongxi\RSS\.worktrees\readme-quickstart-redesign' push origin codex/readme-quickstart-redesign:master
```

Expected: `master` 快进到 README 重构提交
