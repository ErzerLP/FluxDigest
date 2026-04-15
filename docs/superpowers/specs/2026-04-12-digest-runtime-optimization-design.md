# FluxDigest 日报运行链路优化设计

> 用户已明确授权：本轮直接采用推荐方案实施，不再逐段等待确认。

## 目标

1. 默认主模型切到 `MiniMax-M2.7`。
2. 当主模型出现超时、网关错误等可恢复失败时，自动降级到 `mimo-v2-pro`。
3. 把日报中的“单篇文章处理”从串行改成有上限的自适应并发。
4. 修复当前已确认的结构化输出脆弱点，避免单篇 dossier 的 JSON 字段漂移拖垮整条日报。
5. 保持日报最终汇总仍为单次生成，确保汇总输入稳定、顺序可追踪。

## 现状问题

- `internal/service/runtime_processing_runner.go` 的 `ProcessPending` 串行遍历 Miniflux 条目，导致 30+ 篇文章时整体耗时过长。
- 真实运行中已观察到 `reading_value` 被模型输出为对象，`internal/adapter/llm/dossier_builder.go` 当前只接受 string，整条日报任务会直接 retry。
- worker 当前只绑定单模型；即使主模型偶发网络/服务错误，也没有自动降级链路。

## 推荐方案

### A. 模型链路

- 默认模型改为 `MiniMax-M2.7`。
- 新增 worker 运行时 fallback model 列表，默认包含 `mimo-v2-pro`。
- worker 启动时为主模型与 fallback 模型预构建 invoker 链。
- `chatModelInvoker.Generate` 在遇到 timeout、5xx、连接重置等可恢复错误时自动切下一个模型。
- 结构化输出解析不通过时，优先通过更鲁棒的 JSON 归一化兜底；只有 transport/availability 错误才走模型降级，避免无意义重复请求。

### B. 日报文章处理并发

- 在 `RuntimeProcessingRunner.ProcessPending` 中改为“去重后并发处理”。
- 并发度采用：
  - 显式配置优先；
  - 否则根据 `worker.concurrency` 与文章数自适应计算；
  - 上限收敛到安全值，避免一下子把代理/LLM 打爆。
- 处理结果按原始 entry 顺序回填，确保后续 digest planner 输入稳定可复现。
- 任一 article 失败时，收敛为首个错误并停止继续派发，避免脏扩散。

### C. 结构化输出鲁棒化

- 为 dossier builder 增加“宽松字符串”解析能力：string / string[] / object 都能归一成 string。
- 先修复已确认会爆的 `reading_value`，并复用同一归一化类型处理其他高风险文本字段。
- 这样即使 `MiniMax-M2.7` 输出轻微 schema 漂移，也不会直接炸整条日报。

## 边界与不做

- 不触碰 `deployments/compose/docker-compose.yml`。
- 不改变日报“先单篇处理，再整体汇总”的主流程。
- 不在本轮扩张 WebUI 配置面；优先保证 worker 运行稳定与真实链路跑通。

## 验证策略

1. 单元测试：
   - 并发 runner 的结果完整性、顺序稳定性、错误收敛。
   - dossier 宽松 JSON 解析。
   - fallback model 链在主模型报错时可切换。
2. 全量 `go test -count=1 ./...`。
3. 本地构建 release 包。
4. 服务器部署后接真实 Miniflux + 真实 OpenAI-compatible API 跑日报，确认 `daily_digests` 落库。
