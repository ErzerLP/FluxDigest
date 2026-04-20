# FluxDigest 安装器参考

本文档说明根目录 `install.sh` 的高级用法。对大多数用户来说，直接运行下面这一条即可：

```bash
bash install.sh
```

## 常用命令

### 交互式安装（推荐）

```bash
bash install.sh
```

### 直接指定安装动作

```bash
bash install.sh --action install --profile full --stack-dir /opt/fluxdigest-stack --host 192.168.50.10 --force
bash install.sh --action upgrade --stack-dir /opt/fluxdigest-stack
bash install.sh --action rollback --stack-dir /opt/fluxdigest-stack --release-id 20260415070001
bash install.sh --action status --stack-dir /opt/fluxdigest-stack
```

## 支持的 action

- `install`：安装或重装指定 profile
- `upgrade`：在保留现有数据和主要配置的前提下升级当前部署
- `rollback`：回滚到指定 release
- `status`：显示当前部署状态；如果敏感摘要文件不可读，会自动回退为脱敏输出

## 支持的 profile

- `full`
- `fluxdigest-miniflux`
- `fluxdigest-halo`
- `fluxdigest-only`

## 常用参数

- `--action <name>`：指定安装动作
- `--profile <name>`：指定部署组合
- `--stack-dir <dir>`：指定安装目录
- `--release-id <id>`：用于回滚时指定目标 release
- `--host <value>`：安装摘要中显示的访问地址
- `--force`：允许覆盖已有生成文件

## 输出文件

默认安装目录下的关键文件：

```text
/opt/fluxdigest-stack/.env
/opt/fluxdigest-stack/docker-compose.yml
/opt/fluxdigest-stack/install-summary.txt
/opt/fluxdigest-stack/releases/
```

其中 `install-summary.txt` 是最重要的摘要文件，包含：

- 面板入口
- 默认账号密码
- 数据库连接信息
- 当前 release 与镜像 tag

> `install-summary.txt` 默认按敏感信息文件处理。普通用户执行 `bash install.sh --action status ...` 时，如果没有读取该文件的权限，安装器会自动输出脱敏状态摘要；需要查看完整账号密码时，再使用 `sudo cat /opt/fluxdigest-stack/install-summary.txt`。

## 推荐使用方式

- 第一次部署：`bash install.sh`
- 日常升级：`bash install.sh --action upgrade --stack-dir /opt/fluxdigest-stack`
- 查看当前状态：`bash install.sh --action status --stack-dir /opt/fluxdigest-stack`
- 明确回滚版本：`bash install.sh --action rollback --stack-dir /opt/fluxdigest-stack --release-id <release-id>`
