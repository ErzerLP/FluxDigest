# Digest Runtime Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将日报运行链路优化为“MiniMax-M2.7 主模型 + mimo-v2-pro 自动降级 + 自适应并发单篇处理 + 结构化 JSON 鲁棒解析”，并完成真实部署验证。

**Architecture:** worker 在启动期构建多模型 invoker 链，单篇文章处理阶段通过有上限的并发 worker 池加速，dossier 解析层增加宽松字段归一化，最终 digest 仍按稳定候选顺序汇总生成。

**Tech Stack:** Go, GORM, Asynq, Eino/OpenAI-compatible API, PostgreSQL, Redis, ssh-manager

---

### Task 1: 完成 dossier 宽松 JSON 解析

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\adapter\llm\dossier_builder.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\adapter\llm\dossier_builder_test.go`

- [ ] 先补失败测试，覆盖 `reading_value` 为 object / array 时仍能解析。
- [ ] 运行目标测试，确认先红。
- [ ] 实现宽松字符串归一化类型并接到 dossier 输出结构。
- [ ] 重跑目标测试，确认转绿。

### Task 2: 完成 worker 多模型 fallback 链

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\config\config.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\service\runtime_config_service.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\cmd\rss-worker\main.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\cmd\rss-worker\main_test.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\config\config_test.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\service\runtime_config_service_test.go`

- [ ] 先补失败测试，覆盖默认模型切到 `MiniMax-M2.7`、fallback model 透传、主模型报可恢复错误时切到备用模型。
- [ ] 运行相关测试，确认先红。
- [ ] 增加 fallback model 配置读取与 worker invoker 链实现。
- [ ] 重跑测试，确认转绿。

### Task 3: 完成自适应并发文章处理

**Files:**
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\service\runtime_processing_runner.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\service\runtime_processing_runner_test.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\internal\config\config.go`
- Modify: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation\configs\config.example.yaml`

- [ ] 先补失败测试，覆盖并发结果完整、顺序稳定、错误短路、自适应并发度计算。
- [ ] 运行目标测试，确认先红。
- [ ] 实现 article worker pool + 自适应 parallelism。
- [ ] 重跑测试，确认转绿。

### Task 4: 全量验证与真实部署

**Files:**
- Verify only: `D:\Works\guaidongxi\RSS\.worktrees\fluxdigest-a1-content-asset-foundation`

- [ ] 运行 `go test -count=1 ./...`。
- [ ] 本地构建前后端产物。
- [ ] 用 ssh-manager 部署到 `test` 服务器。
- [ ] 接真实 Miniflux + 真实 LLM 跑正式日报，确认 `daily_digests.digest_date='2026-04-12'` 或新的测试日期落库。
