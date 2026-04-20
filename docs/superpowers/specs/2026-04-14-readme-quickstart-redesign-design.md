# README 快速开始与首页重构设计

- 日期：2026-04-14
- 主题：README 首页重构、快速开始入口重写、部署文档导航收敛
- 设计状态：已确认，待进入实现计划

## 1. 背景

当前 `README.md` 的核心问题不是“信息太少”，而是首页承担了过多职责：项目介绍、系统架构、配置枚举、部署说明、开发命令、目录结构全部堆叠在同一入口页中，导致首次访问者很难在短时间内回答三个最基本的问题：

1. FluxDigest 是做什么的。
2. 我应该从哪里开始部署或试用。
3. 已有脚本与详细文档分别对应什么场景。

结合现状检查，当前仓库已经存在可复用的脚本与文档资产：

- `deploy/scripts/deploy-systemd.sh`
- `deploy/scripts/upgrade-systemd.sh`
- `deploy/scripts/rollback-systemd.sh`
- `scripts/smoke-compose.ps1`
- `docs/deployment/full-stack-ubuntu.md`
- `docs/deployment/fluxdigest-systemd.md`
- `docs/deployment/integration-setup.md`
- `docs/api/open-api-guide.md`

但 README 没有把这些资产组织成“产品入口页 + 快速开始 + 文档导航”的清晰结构，同时还混入了旧信息（例如过时的渠道命名、旧接口、旧页面状态），使用户在首页停留后反而更困惑。

## 2. 目标

本次重构目标是把 `README.md` 从“堆满内容的首页手册”改造成“强入口导向的产品首页 + 技术导航页”，满足以下要求：

- 首次用户在 10 秒内看懂 FluxDigest 的定位、输入、处理和输出。
- 首页必须有真正可执行的“快速开始”入口，而不是要求用户先通读长文档。
- 首页同时兼顾两类读者：
  - 首次部署 / 自托管用户（优先）
  - 开发者 / 二次集成者（次级）
- 首页明确区分：
  - 完整环境部署入口
  - 仅部署 FluxDigest 的脚本入口
  - 本地 smoke / 开发验证入口
- 详细部署、联调、接口说明下沉到 `docs/`，README 只负责高价值导航。
- 清理首页中的旧信息与误导性描述。

## 3. 非目标

本轮不做以下事项：

- 不重构现有部署脚本逻辑。
- 不新增真正的“完整环境一键安装器”。
- 不全面重写所有部署文档正文。
- 不调整后端 API、WebUI、数据库或发布链路实现。

说明：用户对“没有一键脚本”有明确感受，但当前仓库实际具备的是 **FluxDigest 本体的 systemd 一键部署脚本** 与 **本地 smoke 脚本**，并没有已经落地的“完整环境一键安装器”。因此 README 必须 **诚实呈现当前能力**：将“脚本入口”写清楚，而不是在首页虚构不存在的完整安装器。

## 4. 设计原则

### 4.1 首页优先解决三个问题

README 第一屏必须优先回答：

1. 这项目是什么。
2. 怎么最快跑起来。
3. 更详细的文档和接口说明在哪里。

### 4.2 首页不是完整手册

README 不再承担完整部署手册职责。系统架构、升级回滚、环境变量细节、深入目录结构等内容不应占据首页前半部分，应下沉到专门文档或 README 后半段开发者区。

### 4.3 先命令 / 入口，再解释

快速开始部分应以“入口型命令 / 入口型文档链接”为优先，不再把部署原理、配置说明放在命令之前。

### 4.4 对现状保持诚实

首页必须明确当前真实存在的脚本入口：

- 完整环境：当前走 `docs/deployment/full-stack-ubuntu.md`（文档入口）
- 仅部署 FluxDigest：`deploy/scripts/deploy-systemd.sh`
- 本地 smoke：`scripts/smoke-compose.ps1`

也就是说，本轮 README 会强化“快速开始”，但不会把当前并不存在的完整安装器包装成“一键脚本”。

## 5. README 新信息架构

新的 `README.md` 采用“两层节奏”：上半部分面向首次用户，下半部分面向开发者。

### 5.1 首页第一屏（首次用户）

#### A. 标题 + 一句话介绍

使用一段足够直白、能快速建立心智的说明：

> FluxDigest 会从 Miniflux 拉取 RSS 文章，调用 AI 做翻译、分析和摘要，自动生成“每日汇总日报”，并可发布到 Halo 或通过 API 提供给其他系统。

这句话必须同时包含：

- 输入：Miniflux / RSS
- 处理：AI 翻译、分析、摘要
- 输出：日报、发布、API

#### B. 核心能力（3–4 条）

仅保留最关键的能力，不展开内部实现细节：

- 单篇文章翻译 / 分析 / 摘要
- 每日汇总日报自动生成
- WebUI 在线配置 LLM / Miniflux / 发布通道
- 发布到 Halo + 开放 API 对外消费

#### C. 快速开始（首页核心）

分成三个入口块：

1. **完整环境快速开始（推荐）**
   首页只给非常短的说明和入口链接，指向 `docs/deployment/full-stack-ubuntu.md`。这里强调“适合第一次部署完整环境”。

2. **只部署 FluxDigest（已有依赖时）**
   直接展示 `deploy/scripts/deploy-systemd.sh` 的命令入口，并说明适用前提：PostgreSQL / Redis / Miniflux / Halo 已就绪。

3. **本地 smoke / 开发验证**
   直接展示 `scripts/smoke-compose.ps1` 的使用入口，方便开发者快速自测。

注意：这里的“快速开始”并不是把所有安装细节塞进 README，而是让用户可以立刻看到最短路径和对应入口。

#### D. 部署完成后的入口

首页提供部署完成后的常见访问入口示意：

- FluxDigest WebUI
- Miniflux 后台
- Halo 后台
- 开放接口文档
- OpenAPI 文件

#### E. 最常见使用流程

用 4 步说明“装完之后怎么用”：

1. 在 Miniflux 后台添加 RSS。
2. 在 FluxDigest WebUI 配置 LLM / Miniflux / 发布通道。
3. 手动触发或等待定时日报。
4. 在 Halo 或 API 中查看结果。

### 5.2 首页第二层（文档导航）

将 README 中所有“继续深入”的阅读路径集中成导航区：

- 完整环境部署：`docs/deployment/full-stack-ubuntu.md`
- systemd 部署 / 升级 / 回滚：`docs/deployment/fluxdigest-systemd.md`
- 联调配置：`docs/deployment/integration-setup.md`
- 开放接口总览：`docs/api/open-api-guide.md`
- OpenAPI：`api/openapi/openapi.yaml`

### 5.3 README 后半段（开发者区）

开发者区保留，但明显后移，并收敛成短格式：

- 本地开发命令
- 测试命令
- 三进程结构简述
- 目录结构（简版）
- 扩展点入口（可选）

开发者区的存在是为了不牺牲仓库的技术可用性，但不能再压住首页主节奏。

## 6. 快速开始的具体写法约束

### 6.1 完整环境快速开始

README 里应呈现为：

- 适用人群：第一次部署完整环境
- 入口：跳转 `docs/deployment/full-stack-ubuntu.md`
- 结果预期：最终能访问 WebUI / Miniflux / Halo

这一块不应伪装为“已有完整安装脚本”。当前仓库没有真正的 full-stack installer，因此只能把它设计成“文档优先的快速入口”。

### 6.2 仅部署 FluxDigest

README 里应直接展示脚本入口命令，例如：

- `sudo ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest`

并配套一句简洁说明：

- 适合 PostgreSQL、Redis、Miniflux、Halo 已准备完成的用户
- 脚本负责构建、安装、切换 release、更新 systemd 服务并做健康检查

### 6.3 本地 smoke

README 里应直接展示：

- `./scripts/smoke-compose.ps1`

并明确它是开发验证入口，不是生产部署入口。

## 7. 应删除或下沉的首页内容

以下内容不应继续占据 README 首页前半段：

- 冗长系统架构解释
- 大段环境变量枚举
- systemd 升级 / 回滚细节
- 逐项 API 列表
- 详细目录树
- 过多内部实现说明

处理方式：

- API 细节 → `docs/api/open-api-guide.md`
- 部署细节 → `docs/deployment/*`
- 目录与开发说明 → README 后半段开发者区

## 8. 需要同步清理的旧信息

本轮 README 重构时，应顺手修正以下旧信息：

- 旧的发布渠道命名或错误别名（例如 `holo`）
- 已不属于当前主契约的旧接口描述
- 已经过时的页面状态描述（例如“某配置页仍是占位”的旧说法）
- 过长且不精确的“关键配置项概览”

原则：首页只保留首次用户最关心的少量配置背景；详细配置入口统一交给部署文档与 env 示例。

## 9. 预期改动文件

### 必改
- `README.md`

### 需同步核对链接与命名一致性的文件
- `docs/deployment/full-stack-ubuntu.md`
- `docs/deployment/fluxdigest-systemd.md`
- `docs/deployment/integration-setup.md`
- `docs/api/open-api-guide.md`

说明：这轮不要求全面重写上述文档正文，但要保证 README 中的命名、入口和跳转关系与这些文档一致。

## 10. 验收标准

本次完成后，应满足：

1. README 首页 10 秒内能看懂项目用途。
2. README 首页有真正的“快速开始”区域，并清晰区分：
   - 完整环境入口
   - 仅部署 FluxDigest 的脚本入口
   - 本地 smoke 入口
3. README 首页不再像完整手册，详细内容下沉到 docs。
4. README 中的旧信息被清理，不再出现明显过时或误导性描述。
5. README 仍保留开发者入口，但不会压住首次用户的主阅读路径。
6. 首页中的脚本/文档入口与仓库现状一致，不虚构不存在的完整安装器。

## 11. 实施边界

实现阶段默认以 `README.md` 为主改文件，必要时只做少量文档导航同步；不在本轮修改部署脚本逻辑。
