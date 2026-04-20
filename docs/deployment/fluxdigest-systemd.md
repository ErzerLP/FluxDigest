# FluxDigest Systemd 正式部署与运维

## 文档适用范围
本文档面向首次自部署 FluxDigest 的运维人员，聚焦 systemd 服务的目录布局、配置文件、部署/升级/回滚框架、日志与常见问题。假定部署环境已有 `systemd`、`go`、`npm` 等基础工具，并且准备在 Linux 服务器上以 `/opt/fluxdigest` 为默认安装基准。

## 安装目录结构
- **应用根目录**：默认 `APP_ROOT=/opt/fluxdigest`（可通过 `--app-root` 修改）。所有 release 以时间戳命名，位于 `/opt/fluxdigest/releases/<timestamp>`；脚本会确保 `/opt/fluxdigest/current` 是指向当前 release 的软链接。每个 release 包含 `bin/`（`rss-api/worker/scheduler`）、`web/dist`、`migrations/` 等运行产物。
- **current 机制**：`deploy/scripts/deploy-systemd.sh` 先生成新 release，再 `ln -sfn` 指向 `current`，然后在 systemd unit 里引用 `/opt/fluxdigest/current`。这个机制与 `rollback` 共用同一套 `current` 软链切换逻辑。切换时会调用 `systemctl daemon-reload`、`enable --now`、`restart` 等命令。
- **环境变量文件**：默认 `/etc/fluxdigest/fluxdigest.env`（可通过 `--env-file` 指定其他路径），由 `deploy-systemd.sh` 从 `deploy/systemd/fluxdigest.env.example` 复制并注入首次部署时已导出的 `APP_*` 变量（`GIN_MODE`、`APP_HTTP_PORT`、`APP_DATABASE_DSN` 等）。脚本会用 `install -m 0640` 创建目录并设置 root:group，确保 `fluxdigest` 用户可读。

## env 文件说明与安全项
`deploy/systemd/fluxdigest.env.example` 是 env 模板，内含默认值和说明：
- Proxy、端口与后端地址：`GIN_MODE=release`、`APP_HTTP_PORT=18088`、`APP_DATABASE_DSN`、`APP_REDIS_ADDR` 等属于最基础的网络/数据源参数。
- 作业与 LLM：`APP_JOB_API_KEY`、`APP_LLM_API_KEY`、`APP_LLM_MODEL`、`APP_LLM_TIMEOUT_MS` 等需要各自服务凭证；`APP_LLM_FALLBACK_MODELS` 支持输出代替模型。
- 发布器：`APP_PUBLISH_CHANNEL=halo`（建议显式设置）/`APP_PUBLISH_OUTPUT_DIR`/`APP_PUBLISH_HALO_BASE_URL`/`APP_PUBLISH_HALO_TOKEN` 控制 markdown 或 Halo 输出目标。
- 安全密钥：`APP_ADMIN_SESSION_SECRET` 与 `APP_SECRET_KEY` 必须在私有渠道生成高强度随机值；部署脚本不会覆盖已有值，只在首次创建 env 文件时使用示例值。
- 小技巧：先在 shell 中 `export` 你希望留在 env 的变量，再运行 `deploy-systemd.sh`，脚本会将当前 shell 中已定义的这些值写入 `/etc/fluxdigest/fluxdigest.env`。

## 首次部署
1. **准备**：在源码树（`deploy/scripts` 所在路径的上两级）执行 `./deploy/scripts/deploy-systemd.sh`。脚本默认会 `npm ci`、`npm run build`、`go build`，然后复制产物到 `/opt/fluxdigest/releases/<timestamp>`。
2. **关键参数**：
   - `--env-file /etc/fluxdigest/fluxdigest.env`：指定共享 env 位置，脚本会先复制 `fluxdigest.env.example`，再填入当前 shell 的 `APP_*` 变量。
   - `--release-retention 5`：指定保留最近 5 个 release。设为 `0` 可关闭自动清理，便于调试和回滚。
   - `--skip-build`：跳过 go/npm 构建，直接使用 `.build/systemd/bin/...` 和 `web/dist`；适用于已有编译好的产物（通常在 CI 产物解包后手动填充）。
3. **部署行为**：
   - `deploy-systemd.sh` 创建运行用户（`APP_USER`，默认当前 sudo 用户或 `fluxdigest`）和必要目录，确保`APP_PUBLISH_OUTPUT_DIR` 也存在。
   - 会渲染 `deploy/systemd/*.service.tpl`（三个模板分别对应 `fluxdigest-api.service`、`fluxdigest-worker.service`、`fluxdigest-scheduler.service`），用 `CURRENT_DIR`、`ENV_FILE`、`APP_USER/APP_GROUP` 替换占位符，并安装到 `/etc/systemd/system`。
   - `systemctl daemon-reload`、`enable --now`、`restart` 三个服务，再用 `curl http://127.0.0.1:${APP_HTTP_PORT:-8080}/healthz` 做健康探测；失败时会回退到先前的 `current`。
   - 完成后会输出 `current release: /opt/fluxdigest/releases/<timestamp>` 以及提示删除旧 release（通过 `--release-retention` 控制）。
4. **常用执行示例**：
   ```bash
   ./deploy/scripts/deploy-systemd.sh --app-root /opt/fluxdigest --env-file /etc/fluxdigest/fluxdigest.env --release-retention 5
   ```
   或跳过构建：
   ```bash
   ./deploy/scripts/deploy-systemd.sh --skip-build
   ```

## 升级
- `deploy/scripts/upgrade-systemd.sh` 只是调用上文的 `deploy-systemd.sh`，并原样透传 `--app-root`/`--env-file`/`--release-retention`/`--skip-build` 等参数；因此日常升级直接运行 `./deploy/scripts/upgrade-systemd.sh [同 deploy 的参数]`，确保 CI 产物与 env 同步即可。
- 升级的核心逻辑不变：仍然会创建 release 目录、渲染 unit、切换 `current`、重启 3 个服务，并做健康检查。若升级失败，脚本会尝试把 `current` 恢复到旧 release 并重新加载 systemd。

## 回滚
- `./deploy/scripts/rollback-systemd.sh` 默认自动回滚到比 `current` 更早的 release。它会：
  1. 确保 `/opt/fluxdigest/current` 是合法的 release，且 `/opt/fluxdigest/releases` 存在。
  2. 按时间戳倒序列出 release，跳过缺失 `bin/rss-*` 的目录；也可通过 `--release-id <timestamp>` 指定目标 release，脚本会验证目标与 `current` 不一致。
  3. `ln -sfn` 目标 release 到 `current`，`systemctl daemon-reload` + `restart fluxdigest-*.service`，再执行与 deploy 相同的 health check。如果任一步失败，会尝试恢复到原来 release 并写 `systemctl status` 供排查。
- 可选参数：`--env-file`（与 deploy 保持一致以读取 `APP_HTTP_PORT` 等）、`--app-root` 切换目录（比如非默认 `/opt/fluxdigest`）。

## Release 清理与保留逻辑
- `deploy-systemd.sh` 中的 `cleanup_old_releases` 函数根据 `--release-retention`（默认 5）决定清理旧 release：只保留最近 `n` 个目录，跳过当前 `current` 指向的 release，并避免删除非 `/opt/fluxdigest/releases/` 目录。
- 设 `--release-retention 0` 可关闭自动清理，适合需要手动分析回滚或保留历史日志时使用；否则清理命令会在部署成功后自动执行，保持磁盘空间。
- 清理执行时先列出符合 `YYYYMMDDHHMMSS` 格式的目录，用 `find`、`sort -r`，然后从第 `n`+1 个起 `rm -rf`，并在日志中输出每个删除的 timestamp。

## 常用 systemctl / journalctl 命令
| 目的 | 命令示例 | 说明 |
| --- | --- | --- |
| 启动/启用服务 | `sudo systemctl enable --now fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service` | 脚本默认已经执行一次，可用于部署后手动重启。 |
| 重启单个服务 | `sudo systemctl restart fluxdigest-worker.service` | 仅重启 worker，不影响其他两个服务。 |
| 查看状态 | `sudo systemctl status fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service` | 获取 unit 状态、启动日志。 |
| 跟踪实时日志 | `sudo journalctl -u fluxdigest-scheduler.service -f` | 同样适用于 api/worker。 |
| 查看最近日志 | `sudo journalctl -u fluxdigest-api.service --since 10 minutes ago` | 结合 `--since/--until` 定位请求或错误。 |
| 验活失败排查 | `curl http://127.0.0.1:${APP_HTTP_PORT:-8080}/healthz` | 使用 env 里 `APP_HTTP_PORT` 的值模拟脚本的 health check。 |

## 常见运维问题
1. **部署卡在健康检查**：确认 env 中 `APP_HTTP_PORT` 与实际监听端口一致，且数据库/Redis/LLM 连接可达；可用 `curl` 模拟脚本的 `HEALTH_ENDPOINT`。
2. **权限或目录不存在**：`deploy-systemd.sh` 会尝试 `install -d` 创建 `/opt/fluxdigest/releases`、`APP_PUBLISH_OUTPUT_DIR` 与 env 目录，但仍需确保 sudo 权限、`APP_USER` 可以读取 `fluxdigest.env`（默认 root/group）。
3. **`--skip-build` 报错缺文件**：意味着 `.build/systemd/bin/rss-*` 或 `web/dist/index.html` 缺失；只能在事先执行过完整构建并将产物复制到 `${SOURCE_DIR}/.build/systemd` 后才使用此参数。
4. **回滚脚本找不到 release**：`rollback-systemd.sh` 会跳过 `bin/rss-*` 不全的目录，且不会回滚到当前已指向的 release。在 `releases` 里手动确认 timestamp 后，可带 `--release-id` 显式指定。
5. **自动清理误删目标**：`--release-retention` 设为 0 可暂停清理；部署目录里保持至少两个 release，避免 `current` 被清理后无法回滚。
6. **env 文件参数被覆盖**：`deploy-systemd.sh` 只有在 env 文件不存在时才从示例复制，后续再次部署不会重写已有值；必要修改请 `sudo vim /etc/fluxdigest/fluxdigest.env` 后 `systemctl restart`。

以上内容基于 `deploy/scripts/deploy-systemd.sh`、`deploy/scripts/upgrade-systemd.sh`、`deploy/scripts/rollback-systemd.sh` 以及 `deploy/systemd/fluxdigest.env.example` 的真实行为和配置模板。请按步骤执行，持续监控 `journalctl` 和 health check 反馈。
