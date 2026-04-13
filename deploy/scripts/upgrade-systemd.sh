#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_SCRIPT="${SCRIPT_DIR}/deploy-systemd.sh"

usage() {
  cat <<'EOF'
Usage: upgrade-systemd.sh [options]

说明：
  标准升级入口。默认行为是调用 deploy-systemd.sh 完成构建、发布、切换 current、重启与健康检查。
  主要参数会原样透传给 deploy-systemd.sh（如 --app-root / --app-user / --env-file / --service-dir / --source-dir / --release-retention / --skip-build）。

Examples:
  ./deploy/scripts/upgrade-systemd.sh
  ./deploy/scripts/upgrade-systemd.sh --skip-build --release-retention 7

  -h, --help             显示帮助
EOF
}

if [[ ! -x "${DEPLOY_SCRIPT}" ]]; then
  echo "未找到可执行部署脚本: ${DEPLOY_SCRIPT}" >&2
  exit 1
fi

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

exec "${DEPLOY_SCRIPT}" "$@"
