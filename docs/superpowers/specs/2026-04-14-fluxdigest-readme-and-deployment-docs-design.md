# FluxDigest README 与部署教程设计说明

- 日期：2026-04-14
- 设计主题：面向首次部署用户的 README 重构与完整环境部署文档
- 工作区：`D:\Works\guaidongxi\RSS\.worktrees\readme-deployment-docs`
- 当前定位：单用户、个人自用、平台优先、Ubuntu 22.04 / 24.04 x86_64 自部署场景

## 1. 设计目标

为 FluxDigest 重写 README，并补齐一套真正可执行的部署教程，使首次接触项目的用户可以从“看到仓库首页”顺利走到“在一台新 Ubuntu 服务器上部署好 Miniflux + Halo + FluxDigest，并完成首次联调”。

本轮文档重构重点解决以下问题：

1. README 信息混杂，首次部署路径不够清晰
2. 开发说明、生产部署说明、外部组件说明边界不够明确
3. FluxDigest / Miniflux / Halo 之间的职责关系没有被讲清楚
4. systemd 脚本、升级/回滚脚本、旧 release 清理机制虽然已经实现，但缺少用户可直接照着执行的文档
5. 当前文档需要更明确地区分“仓库内开发 smoke 能力”和“正式部署能力”

## 2. 已确认的需求边界

### 2.1 目标读者

本轮文档的第一优先读者是：

- 第一次接触 FluxDigest 的部署用户
- 使用 Ubuntu 22.04 / 24.04 x86_64 的自托管用户
- 希望同时部署 Miniflux、Halo、FluxDigest 的个人用户
- 对 Go 项目内部实现细节不敏感，更关注“怎么装、怎么配、怎么验收”的用户

### 2.2 部署范围

文档需要覆盖完整环境，而不是只写 FluxDigest 本体：

- PostgreSQL
- Redis
- Miniflux
- Halo
- FluxDigest API / Worker / Scheduler / WebUI
- LLM 接入配置
- Miniflux / Halo / FluxDigest 首次联调

### 2.3 文档主线

采用“双路线，但以脚本优先为主线”的方案：

- 主线：FluxDigest 本体正式部署以现有 `systemd` 脚本为准
- 辅线：对 Miniflux 与 Halo 提供手工部署步骤，并尽量遵循官方推荐方式
- 对不能脚本化的一次性初始化步骤，提供明确的手工操作说明

### 2.4 明确不在本轮扩展范围内的内容

以下内容本轮可以提及，但不作为文档主目标：

- 多用户 / 多租户部署指南
- Kubernetes / Helm 部署方案
- 完整的性能调优手册
- 面向二次开发者的深度架构设计文档
- 自动化一键安装整套 Miniflux + Halo + FluxDigest 的统一安装器实现

## 3. 总体文档方案选择

最终采用：**方案 C：README 作为总入口 + 分层部署文档**。

### 3.1 选择原因

该方案最适合当前项目状态与目标读者：

- README 负责“让用户快速理解项目并找到正确入口”，避免首页过长失去导航价值
- 详细步骤拆分到部署文档后，更利于维护、升级与校对
- FluxDigest 本体部署、完整环境部署、首次联调是三个不同心智阶段，拆开后更易读
- 未来新增新的发布器、安装方式、运维能力时，可以继续扩展文档树而不需要反复重写 README

## 4. 文档信息架构

本轮文档最终交付应包含以下结构：

### 4.1 `README.md`

README 定位为项目总入口，负责：

- 说明 FluxDigest 是什么
- 说明 Miniflux / FluxDigest / Halo 三者关系
- 展示核心能力、典型工作流与主要访问入口
- 说明支持的部署方式
- 给出“首次部署用户”的推荐阅读顺序
- 链接到更详细的部署文档与联调文档

README 不承担逐命令写完全部部署步骤的职责。

### 4.2 `docs/deployment/full-stack-ubuntu.md`

这是面向首次部署用户的主教程，应作为 README 的首推入口。

需要覆盖：

1. 服务器前置要求
2. Docker / Docker Compose 安装
3. PostgreSQL / Redis 准备
4. Miniflux 部署与初始化
5. Halo 部署与初始化
6. FluxDigest 拉代码、配置、systemd 部署
7. 首次登录 WebUI
8. 首次配置 Miniflux / LLM / Halo
9. 触发一次真实日报并验证结果

### 4.3 `docs/deployment/fluxdigest-systemd.md`

这是 FluxDigest 本体的正式部署与运维文档。

需要覆盖：

- `deploy/scripts/deploy-systemd.sh`
- `deploy/scripts/upgrade-systemd.sh`
- `deploy/scripts/rollback-systemd.sh`
- `/etc/fluxdigest/fluxdigest.env` 的生成与维护
- `current` 软链与 `releases/<timestamp>` 目录结构
- release 保留数量与自动清理机制
- 常用 `systemctl` / `journalctl` 运维命令

### 4.4 `docs/deployment/integration-setup.md`

这是首次联调说明文档，重点回答“装好了之后怎么真正用起来”。

需要覆盖：

- 如何在 Miniflux 中添加 RSS 订阅
- 如何在 FluxDigest WebUI 配置 Miniflux
- 如何填写 LLM Base URL / API Key / Model / Timeout
- 如何在 Halo 中创建 PAT 并回填到 FluxDigest
- 如何使用 WebUI 手动重跑日报
- 如何判断文章、dossier、日报、发布结果是否成功

## 5. 各文档应强调的事实边界

### 5.1 FluxDigest 与 Miniflux 的职责边界

文档必须明确：

- Miniflux 负责 RSS 订阅源管理、聚合与文章抓取
- FluxDigest 不负责 RSS 订阅管理本身
- FluxDigest 负责消费 Miniflux 已聚合的文章，并进行翻译、分析、汇总与发布
- WebUI 中应给用户一个“跳转到 Miniflux 后台”的认知路径，但订阅操作仍在 Miniflux 完成

### 5.2 FluxDigest 与 Halo 的职责边界

文档必须明确：

- FluxDigest 负责把日报/文章转换为可发布内容
- Halo 是最终博客发布目标之一
- 当前发布器优先推荐使用 `halo` 通道，而不是遗留命名或模糊别名
- PAT、站点地址、发布状态的来源和校验方式需要写清楚

### 5.3 开发 Smoke 与正式部署边界

文档必须明确：

- `deployments/compose/docker-compose.yml` 主要服务于开发验证 / smoke 场景
- 其中的 `mock-miniflux`、`mock-llm` 不是生产推荐形态
- 正式部署主线应以真实 Miniflux、真实 LLM、真实 Halo、systemd 管理的 FluxDigest 为准

## 6. 文档内容规范

### 6.1 术语统一

本轮文档统一使用以下命名：

- `Halo`：博客系统名称
- `Miniflux`：RSS 聚合器
- `FluxDigest WebUI`：FluxDigest 管理后台
- `daily digest` / `日报`：单日汇总文章
- `article dossier` / `dossier`：单篇文章翻译分析结果

禁止继续混用：

- `Holo`
- `Halo/Holo` 混写
- 不区分“文章处理结果”和“日报”的表述

### 6.2 写作风格

文档应以“首次部署用户”视角书写，每一步尽量包含：

- 为什么要做这一步
- 具体命令或操作入口
- 执行成功后应看到什么现象
- 常见错误时应该检查哪里

### 6.3 事实来源约束

文档内容必须优先依据以下事实来源：

1. 当前 `master` 已有实现与脚本
2. 当前 WebUI / API / systemd 发布链路的真实行为
3. Miniflux 与 Halo 官方安装文档

不能把计划中的能力写成已经具备的能力。

## 7. 本轮交付物设计

### 7.1 README 需要包含的章节

推荐章节顺序：

1. 项目简介
2. 核心工作流
3. 适用场景与当前定位
4. 系统组成
5. 访问入口说明（FluxDigest / Miniflux / Halo）
6. 快速开始导航
7. 完整环境部署入口
8. FluxDigest 正式部署与运维入口
9. 首次联调与使用入口
10. API / OpenAPI / 扩展能力简述
11. 目录结构概览

### 7.2 完整部署文档需要包含的章节

推荐章节顺序：

1. 适用范围与最终目标
2. 服务器规格建议
3. 端口规划建议
4. 基础依赖安装
5. PostgreSQL / Redis 准备
6. Miniflux 部署
7. Halo 部署
8. FluxDigest 部署
9. WebUI 首次登录与默认管理员说明
10. LLM / Miniflux / Halo 首次接入
11. 触发一次日报并验收
12. 常见问题

### 7.3 systemd 运维文档需要包含的章节

推荐章节顺序：

1. 目录布局
2. env 文件说明
3. 首次部署命令
4. 升级命令
5. 回滚命令
6. 旧 release 清理逻辑
7. 服务启停与日志查看
8. 常见运维问题

### 7.4 首次联调文档需要包含的章节

推荐章节顺序：

1. 在 Miniflux 添加 RSS
2. 在 FluxDigest 配置 Miniflux
3. 在 FluxDigest 配置 LLM
4. 在 Halo 创建 PAT
5. 在 FluxDigest 配置 Halo 发布
6. 手动触发日报
7. 检查 API / 数据库 / 发布结果
8. 常见联调失败原因

## 8. 安全与默认值说明

文档必须明确写出：

- FluxDigest 首次安装后的默认管理员用户名/密码均为 `FluxDigest`
- 首次登录后应立即修改
- `APP_ADMIN_SESSION_SECRET` 与 `APP_SECRET_KEY` 必须替换为高强度随机值
- `APP_JOB_API_KEY`、Halo PAT、LLM API Key、Miniflux Token 不应使用示例值直接上线
- 如果服务器需要代理访问外部 LLM，需明确配置 `http_proxy` / `https_proxy`

## 9. 验收标准

当本轮文档完成后，应满足以下结果：

1. 首次进入仓库的用户，只看 README 就知道应该先读哪篇文档
2. 用户可以按照完整部署文档在 Ubuntu 22.04 / 24.04 上完成整套环境部署
3. 用户可以按照联调文档完成 Miniflux / LLM / Halo 接入
4. 用户可以按照 systemd 文档完成部署、升级、回滚、日志排查
5. 文档中不再出现 `Holo` 这类混用命名
6. 文档中的命令、文件路径、环境变量名与当前代码实现一致

## 10. 后续实现顺序

本轮文档实现建议按以下顺序推进：

1. 先重写 `README.md`，建立清晰入口
2. 再补 `docs/deployment/full-stack-ubuntu.md`
3. 再补 `docs/deployment/fluxdigest-systemd.md`
4. 最后补 `docs/deployment/integration-setup.md`
5. 完成后逐项校对 README 链接、脚本名、环境变量名、默认值与当前实现的一致性
