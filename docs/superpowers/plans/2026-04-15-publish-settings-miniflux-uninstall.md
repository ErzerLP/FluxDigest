# Publish Settings / Miniflux 已读同步 / 卸载器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成发布设置页、手动生成日报、Miniflux 已读回写与一键卸载能力。

**Architecture:** 沿用现有 admin config + runtime snapshot + worker 处理链，不重做主架构。发布设置页同时编排 publish profile 与 scheduler profile；文章发布策略在 dossier 物化阶段落库；Miniflux 已读在 processing runner 成功后批量回写。

**Tech Stack:** Go, Gin, GORM, React, React Query, react-hook-form, bash installer

---

### Task 1: 失败测试覆盖后端配置与 admin 路由
- [ ] 补 `internal/service/admin_config_service_test.go`：覆盖 publish 新字段与 scheduler snapshot/update。
- [ ] 补 `internal/app/api/handlers/admin_handler_test.go`：覆盖 scheduler 更新接口与 admin 手动日报接口。

### Task 2: 失败测试覆盖 Miniflux 与处理链
- [ ] 补 `internal/adapter/miniflux/client_test.go`：验证 `status=unread` 与批量标记已读请求。
- [ ] 补 `internal/service/runtime_processing_runner_test.go`：验证成功处理后批量回写已读。

### Task 3: 失败测试覆盖 dossier 发布策略与来源注入
- [ ] 补 `internal/service/dossier_service_test.go`：验证来源块注入、不同发布策略生成 `draft/pending_review/queued`。

### Task 4: 失败测试覆盖前端发布设置页
- [ ] 补 `web/src/__tests__/publish-config.test.tsx`：覆盖日报时间、文章发布模式、审核模式、手动生成日报按钮。

### Task 5: 失败测试覆盖安装器卸载
- [ ] 新增 `deploy/stack/tests/install_uninstall_smoke.sh`：验证保留数据/清理数据两种路径。

### Task 6: 实现后端与前端
- [ ] 扩展 publish/scheduler config 读写与 runtime snapshot。
- [ ] 新增 admin 手动日报接口。
- [ ] 实现 Miniflux unread 拉取与 mark-read。
- [ ] 实现 dossier 来源块与发布状态初始化。
- [ ] 实现 Publish 设置页面真实交互。

### Task 7: 实现脚本与验证
- [ ] 单脚本交互入口新增 uninstall。
- [ ] stack 安装器实现 uninstall / purge-data。
- [ ] 运行 Go / Web / shell 验证。
