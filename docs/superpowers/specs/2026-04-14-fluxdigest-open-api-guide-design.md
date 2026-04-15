# FluxDigest 开放接口文档设计

- 日期：2026-04-14
- 主题：开放接口总览文档（Open API Guide）
- 设计状态：已确认，待进入实现计划

## 1. 背景

FluxDigest 目前已经具备两类 API 能力：

1. **对外内容消费接口**：供博客、知识库、Bot、自动化脚本等系统读取文章处理结果、单篇 dossier 与最新日报。
2. **管理/运维接口**：供 WebUI、部署联调、管理员排障与手动触发任务使用。

仓库中已存在 `api/openapi/openapi.yaml` 作为机器可读规范，但缺少一份面向人的、可快速导航的“开放接口总览”文档。现有 README 也尚未提供专门的接口文档入口。

## 2. 目标

本次设计目标是新增一份**简洁、面向人的开放接口总览文档**，并在 README 中加入入口，满足以下要求：

- 同时服务两类读者：
  - 第三方集成开发者
  - 项目自用/运维/调试人员
- 文档主线优先展示**内容读取接口**。
- 管理、配置、测试、任务触发相关接口放入**附录**，避免打断对外消费主线。
- 只描述**当前已实现且可确认存在**的接口，不写未落地能力。
- 文档风格为**总览/导航**，而不是冗长教程或逐字段手册。

## 3. 非目标

本次工作不包括：

- 不重写 OpenAPI 规范文件。
- 不把每个 schema 的全部字段逐项解释为人工手册。
- 不承诺任何尚未实现或尚未在 OpenAPI 中声明的接口。
- 不在本轮新增 SDK、Postman Collection 或自动生成文档站点。

## 4. 读者与使用路径

### 4.1 第三方集成开发者

优先阅读：

1. API 基础约定
2. 文章 / dossier / digest 等内容读取接口
3. 如需理解当前启用策略，再阅读 profile 能力查询接口

### 4.2 运维 / WebUI / 联调使用者

除上述内容外，还会继续阅读：

1. 管理状态接口
2. 配置读写接口
3. LLM 测试接口
4. Job 查询与手动触发接口

## 5. 文档落位与文件结构

### 5.1 新增文档

新增文件：`docs/api/open-api-guide.md`

该文档定位为：

- 面向人的接口导航与总览
- 明确哪些接口适合外部消费、哪些接口属于管理附录
- 与 `api/openapi/openapi.yaml` 配套，而非替代它

### 5.2 README 联动

更新 `README.md`，新增一个简短的“接口与集成”入口区块，链接到：

- `docs/api/open-api-guide.md`
- `api/openapi/openapi.yaml`
- 现有部署/联调文档（保持现有文档导航关系）

README 仅负责导航，不复制整份接口文档内容。

## 6. 文档信息架构

`docs/api/open-api-guide.md` 拟采用如下结构：

### 6.1 文档说明

说明：

- 适用对象
- 文档与 OpenAPI 的关系
- 对外读取接口与管理接口的边界
- 只覆盖当前已实现接口

### 6.2 API 基础约定

统一说明最小公共约定，例如：

- Base URL 书写方式
- 返回格式为 JSON
- 鉴权头（如适用）
- 时间格式 / ID / 常见调用约束

这一章只写统一规则，避免各接口章节重复表述。

### 6.3 基础运行与观测接口

在正文前部补充一节基础接口，用于说明系统存活与观测入口：

- `GET /healthz`
- `GET /metrics`

这两项接口不属于内容消费主线，也不属于后台配置接口，因此单独成节，方便运维与集成方快速确认服务健康状态。

### 6.4 对外内容读取接口（主章节）

按“外部系统优先使用”的路径组织：

1. `GET /api/v1/articles`
2. `GET /api/v1/dossiers`
3. `GET /api/v1/dossiers/{id}`
4. `GET /api/v1/digests/latest`

每个接口条目采用统一短格式：

- 接口名称
- 方法 + 路径
- 用途说明
- 关键请求参数
- 返回内容概览
- 使用建议 / 注意事项

### 6.5 运行配置 / 能力查询接口

正文中追加一节描述：

- `GET /api/v1/profiles/{profileType}/active`

原因：该接口虽然不是纯消费接口，但对外部系统理解当前启用的 profile / 输出策略有实际价值，放在正文末尾比完全塞进附录更便于阅读。

### 6.6 附录 A：管理与运维接口

集中收纳以下接口：

- `GET /api/v1/admin/status`
- `GET /api/v1/admin/configs`
- `PUT /api/v1/admin/configs/llm`
- `PUT /api/v1/admin/configs/miniflux`
- `PUT /api/v1/admin/configs/publish`
- `PUT /api/v1/admin/configs/prompts`
- `POST /api/v1/admin/test/llm`
- `GET /api/v1/admin/jobs`
- `POST /api/v1/jobs/daily-digest`

附录明确声明：这些接口主要面向 FluxDigest WebUI、部署联调和日常维护，不建议普通第三方消费方将配置写入接口当作公共协议依赖。

### 6.7 附录 B：错误处理与相关资源

以总览方式列出：

- 常见错误类别（认证失败、参数错误、配置缺失、上游依赖不可用、任务执行失败）
- OpenAPI 规范文件位置
- README / 部署文档入口

## 7. 当前纳入文档的接口清单

本轮文档只覆盖当前已在 `api/openapi/openapi.yaml` 中声明、且已能从代码路径确认的接口：

- `GET /healthz`
- `GET /metrics`
- `GET /api/v1/articles`
- `GET /api/v1/dossiers`
- `GET /api/v1/dossiers/{id}`
- `GET /api/v1/digests/latest`
- `GET /api/v1/profiles/{profileType}/active`
- `GET /api/v1/admin/status`
- `GET /api/v1/admin/configs`
- `PUT /api/v1/admin/configs/llm`
- `PUT /api/v1/admin/configs/miniflux`
- `PUT /api/v1/admin/configs/publish`
- `PUT /api/v1/admin/configs/prompts`
- `POST /api/v1/admin/test/llm`
- `GET /api/v1/admin/jobs`
- `POST /api/v1/jobs/daily-digest`

特别约束：**不得**将当前尚未出现在 OpenAPI 中的测试接口（例如假设性的 Miniflux 测试或 Publish 测试接口）写入文档。

## 8. 内容风格要求

开放接口文档采用“简洁总览”风格：

- 强调接口用途、适用对象、配合关系
- 仅给出必要的请求/响应概览
- 不展开为逐字段参考手册
- 对精细字段定义统一引导至 OpenAPI

这样可以保证：

- 第三方能快速知道先用哪些接口
- 运维侧能快速定位后台能力
- 文档不会与 OpenAPI 形成重复维护负担

## 9. 一致性与维护原则

文档内容的事实来源优先级如下：

1. `api/openapi/openapi.yaml`
2. 当前实际路由/handler 实现
3. WebUI 当前真实调用行为
4. README 与其他人工文档

写作时必须遵守：

- 只写当前真实存在的接口
- 不把规划能力写成现状
- 若人工文档与 OpenAPI 冲突，以已实现接口与 OpenAPI 为准，并在实现阶段同步修正文案

## 10. 验收标准

本次文档工作完成后，应满足：

1. 仓库新增 `docs/api/open-api-guide.md`
2. README 新增接口文档入口
3. 文档主线明确区分：
   - 对外内容读取接口
   - 管理/运维接口附录
4. 文档仅覆盖当前真实存在接口
5. 文档语言简洁，可独立阅读，不依赖读者先看 OpenAPI
6. 文档中明确指向 `api/openapi/openapi.yaml` 作为精细字段与机器可读规范来源

## 11. 实施边界

实现阶段仅涉及文档改动：

- 新增 `docs/api/open-api-guide.md`
- 更新 `README.md`

不改动后端 API、WebUI、OpenAPI schema、数据库结构或部署脚本。
