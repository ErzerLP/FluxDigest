# FluxDigest Open API Guide Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增一份面向人的开放接口总览文档，并在 README 中补充入口，同时确保文档内容严格对齐当前已实现 API。

**Architecture:** 以 `api/openapi/openapi.yaml` 作为事实来源，在 `docs/api/open-api-guide.md` 中按“基础接口 → 内容读取接口 → profile 能力查询 → 管理附录”组织说明；`README.md` 只保留导航入口，不重复详细接口内容。最终通过本地校验、GitHub 推送、测试服务器拉取核对来完成交付。

**Tech Stack:** Markdown、OpenAPI 3.1、PowerShell、Python、Git、GitHub、SSH 服务器工作流

---

## File Map

- Create: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md` — 人工可读的开放接口总览文档
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\README.md` — 仓库首页的接口文档入口
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\api\openapi\openapi.yaml` — 当前 API 事实来源
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\superpowers\specs\2026-04-14-fluxdigest-open-api-guide-design.md` — 已批准设计

### Task 1: 新增开放接口总览文档

**Files:**
- Create: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\api\openapi\openapi.yaml`

- [ ] **Step 1: 运行缺失校验，确认目标文档尚未存在**

Run:

```powershell
@'
from pathlib import Path
path = Path(r'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md')
if not path.exists():
    raise SystemExit('MISSING: open-api-guide.md')
print('UNEXPECTED: file already exists')
'@ | python -
```

Expected: 进程以非 0 退出，并输出 `MISSING: open-api-guide.md`

- [ ] **Step 2: 写入开放接口总览文档**

```powershell
@'
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
- **返回格式**：除 `/metrics` 外，其余接口均返回 JSON
- **鉴权方式**：当前公开接口中，`POST /api/v1/jobs/daily-digest` 需要在请求头中传入 `X-API-Key`
- **时间格式**：日期字段通常使用 RFC 3339 的 `date` / `date-time` 形式
- **事实来源**：若本文档与 `api/openapi/openapi.yaml` 出现差异，以 OpenAPI 与实际实现为准

## 基础运行与观测接口

### 健康检查
- **接口**：`GET /healthz`
- **用途**：确认 API 进程是否存活
- **返回概览**：返回简单的健康状态对象，例如 `status=ok`
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

> 以下接口主要面向 FluxDigest WebUI、部署联调与日常维护，不建议普通第三方内容消费方把这些写接口当作公共集成协议依赖。

### 管理状态与配置读取
- `GET /api/v1/admin/status`：读取 Dashboard 状态、集成连通性摘要、最近运行概况
- `GET /api/v1/admin/configs`：读取当前 LLM、Miniflux、Publish、Prompt 配置快照

### 配置写入接口
- `PUT /api/v1/admin/configs/llm`：更新 LLM 基础配置
- `PUT /api/v1/admin/configs/miniflux`：更新 Miniflux 基础配置
- `PUT /api/v1/admin/configs/publish`：更新发布器配置（当前重点是 Halo / Markdown Export）
- `PUT /api/v1/admin/configs/prompts`：更新 translation / analysis / dossier / digest 提示词配置

### 联调与任务接口
- `POST /api/v1/admin/test/llm`：测试当前草稿 LLM 配置是否能完成连通性检查
- `GET /api/v1/admin/jobs`：查询最近的任务执行记录，支持 `limit`
- `POST /api/v1/jobs/daily-digest`：手动触发一次日报任务；该接口需要 `X-API-Key`

## 附录 B：常见错误与相关资源

### 常见错误类型
- **400**：请求体、query 参数或时间格式不合法
- **401**：`POST /api/v1/jobs/daily-digest` 的 `X-API-Key` 无效
- **500**：内部处理失败，例如配置更新失败、连通性测试失败、任务入队失败
- **503**：当前环境未配置对应 reader / updater / tester / trigger

### 相关资源
- **机器可读 OpenAPI**：`api/openapi/openapi.yaml`
- **仓库入口文档**：`README.md`
- **部署与联调**：`docs/deployment/full-stack-ubuntu.md`、`docs/deployment/fluxdigest-systemd.md`、`docs/deployment/integration-setup.md`
'@ | Set-Content -Path 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md' -Encoding UTF8
```

- [ ] **Step 3: 校验文档包含必需章节与接口路径**

Run:

```powershell
@'
from pathlib import Path
path = Path(r'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md')
text = path.read_text(encoding='utf-8')
required = [
    '# FluxDigest 开放接口总览',
    '## API 基础约定',
    '`GET /healthz`',
    '`GET /metrics`',
    '`GET /api/v1/articles`',
    '`GET /api/v1/dossiers`',
    '`GET /api/v1/dossiers/{id}`',
    '`GET /api/v1/digests/latest`',
    '`GET /api/v1/profiles/{profileType}/active`',
    '`GET /api/v1/admin/status`',
    '`PUT /api/v1/admin/configs/llm`',
    '`POST /api/v1/admin/test/llm`',
    '`POST /api/v1/jobs/daily-digest`',
]
missing = [item for item in required if item not in text]
if missing:
    raise SystemExit('MISSING: ' + ', '.join(missing))
print('PASS: open-api-guide.md covers required sections and routes')
'@ | python -
```

Expected: 输出 `PASS: open-api-guide.md covers required sections and routes`

- [ ] **Step 4: 提交开放接口总览文档**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' add 'docs/api/open-api-guide.md'
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' commit -m "docs: add open api guide"
```

Expected: 生成只包含 `docs/api/open-api-guide.md` 的 commit

### Task 2: 更新 README 的接口与集成入口

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\README.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md`

- [ ] **Step 1: 运行缺失校验，确认 README 还没有开放接口导航**

Run:

```powershell
Select-String -Path 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\README.md' -Pattern 'open-api-guide.md|接口与集成'
```

Expected: 无匹配输出

- [ ] **Step 2: 在 README 中插入“接口与集成”区块**

```powershell
@'
from pathlib import Path
path = Path(r'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\README.md')
text = path.read_text(encoding='utf-8')
anchor = '## 开发 Smoke 与正式部署边界\n'
insert = '''## 接口与集成
- [开放接口总览](docs/api/open-api-guide.md)：面向第三方集成与运维调试的人类可读接口导航。
- [OpenAPI 规范](api/openapi/openapi.yaml)：机器可读 schema、请求/响应与鉴权定义。
- [联调与配置指南](docs/deployment/integration-setup.md)：对接 Miniflux、LLM、Halo 与手动触发任务的操作说明。

'''
if insert in text:
    raise SystemExit('README already updated')
if anchor not in text:
    raise SystemExit('Anchor not found: 开发 Smoke 与正式部署边界')
text = text.replace(anchor, insert + anchor, 1)
path.write_text(text, encoding='utf-8')
print('PASS: README navigation inserted')
'@ | python -
```

Expected: 输出 `PASS: README navigation inserted`

- [ ] **Step 3: 校验 README 链接存在且目标文件可达**

Run:

```powershell
@'
from pathlib import Path
root = Path(r'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs')
readme = (root / 'README.md').read_text(encoding='utf-8')
required_links = [
    'docs/api/open-api-guide.md',
    'api/openapi/openapi.yaml',
    'docs/deployment/integration-setup.md',
]
missing_links = [item for item in required_links if item not in readme]
if missing_links:
    raise SystemExit('README missing links: ' + ', '.join(missing_links))
missing_files = [item for item in required_links if not (root / item).exists()]
if missing_files:
    raise SystemExit('Missing target files: ' + ', '.join(missing_files))
print('PASS: README links and targets verified')
'@ | python -
```

Expected: 输出 `PASS: README links and targets verified`

- [ ] **Step 4: 提交 README 导航更新**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' add 'README.md'
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' commit -m "docs: add api documentation links"
```

Expected: 生成只包含 `README.md` 的 commit

### Task 3: 进行文档一致性校验并完成交付流程

**Files:**
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\docs\api\open-api-guide.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\README.md`
- Reference: `D:\Works\guaidongxi\RSS\.worktrees\open-api-docs\api\openapi\openapi.yaml`

- [ ] **Step 1: 做本地一致性校验**

Run:

```powershell
@'
from pathlib import Path
import yaml
root = Path(r'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs')
openapi = yaml.safe_load((root / 'api/openapi/openapi.yaml').read_text(encoding='utf-8'))
implemented_paths = sorted(openapi['paths'].keys())
text = (root / 'docs/api/open-api-guide.md').read_text(encoding='utf-8')
required_paths = [
    '/healthz',
    '/metrics',
    '/api/v1/articles',
    '/api/v1/dossiers',
    '/api/v1/dossiers/{id}',
    '/api/v1/digests/latest',
    '/api/v1/profiles/{profileType}/active',
    '/api/v1/admin/status',
    '/api/v1/admin/configs',
    '/api/v1/admin/configs/llm',
    '/api/v1/admin/configs/miniflux',
    '/api/v1/admin/configs/publish',
    '/api/v1/admin/configs/prompts',
    '/api/v1/admin/test/llm',
    '/api/v1/admin/jobs',
    '/api/v1/jobs/daily-digest',
]
missing_from_openapi = [item for item in required_paths if item not in implemented_paths]
missing_from_doc = [item for item in required_paths if item not in text]
if missing_from_openapi:
    raise SystemExit('OpenAPI missing paths: ' + ', '.join(missing_from_openapi))
if missing_from_doc:
    raise SystemExit('Guide missing paths: ' + ', '.join(missing_from_doc))
print('PASS: guide and OpenAPI path sets align')
'@ | python -
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' diff --check
```

Expected: 先输出 `PASS: guide and OpenAPI path sets align`，随后 `git diff --check` 无输出

- [ ] **Step 2: 推送当前分支到 GitHub**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' push origin codex/open-api-docs
```

Expected: 远端出现 `codex/open-api-docs`

- [ ] **Step 3: 在测试服务器拉取并核对文档版本**

Run on test server repository `/home/hjx/FluxDigest-admin-config-pages-live`:

```bash
git fetch git@github.com:ErzerLP/FluxDigest.git refs/heads/codex/open-api-docs:refs/remotes/origin/codex/open-api-docs
git checkout codex/open-api-docs
git reset --hard origin/codex/open-api-docs
git rev-parse HEAD
sed -n '1,80p' docs/api/open-api-guide.md
sed -n '1,120p' README.md
```

Expected:
- `git rev-parse HEAD` 输出与本地分支最新提交一致
- `docs/api/open-api-guide.md` 能看到“FluxDigest 开放接口总览”标题
- `README.md` 能看到“接口与集成”区块

- [ ] **Step 4: 完成最终集成与合并**

Run:

```powershell
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' checkout master
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' pull --ff-only origin master
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' merge --ff-only codex/open-api-docs
git -C 'D:\Works\guaidongxi\RSS\.worktrees\open-api-docs' push origin master
```

Expected: `master` 快进到包含开放接口文档的最新提交
