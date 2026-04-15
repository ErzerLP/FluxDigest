#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
README_FILE="${ROOT}/README.md"
FULL_STACK_DOC="${ROOT}/docs/deployment/full-stack-ubuntu.md"
INSTALLER_DOC="${ROOT}/docs/deployment/installer-reference.md"

grep -q '^## Welcome$' "${README_FILE}" || { echo 'README 缺少 Welcome' >&2; exit 1; }
grep -q '^## Feature$' "${README_FILE}" || { echo 'README 缺少 Feature' >&2; exit 1; }
grep -q '^## Quick Start$' "${README_FILE}" || { echo 'README 缺少 Quick Start' >&2; exit 1; }
grep -q 'bash install.sh' "${README_FILE}" || { echo 'README 未展示根安装命令' >&2; exit 1; }
! grep -q '## 这是干嘛的' "${README_FILE}" || { echo 'README 仍包含旧口语标题' >&2; exit 1; }
grep -q 'whiptail' "${FULL_STACK_DOC}" || { echo 'full-stack 文档未说明交互依赖' >&2; exit 1; }
grep -q 'install.sh --action upgrade' "${INSTALLER_DOC}" || { echo 'installer reference 缺少升级示例' >&2; exit 1; }
