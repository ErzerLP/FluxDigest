#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_INSTALL_BIN="${FLUXDIGEST_STACK_INSTALL_BIN:-${ROOT_DIR}/deploy/stack/install.sh}"
WHIPTAIL_BIN="${WHIPTAIL_BIN:-whiptail}"
DEFAULT_STACK_DIR="/opt/fluxdigest-stack"
DEFAULT_HOST="127.0.0.1"
ROOT_ACTION="install"
ROOT_PROFILE=""
ROOT_STACK_DIR="${DEFAULT_STACK_DIR}"
ROOT_RELEASE_ID=""
ROOT_HOST=""
ROOT_FORCE=0
ROOT_NON_INTERACTIVE=0
ROOT_SHOW_HELP=0

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Interactive:
  bash install.sh

Direct actions:
  bash install.sh --action install --profile full --stack-dir /opt/fluxdigest-stack --host 192.168.50.10 --force
  bash install.sh --action upgrade --stack-dir /opt/fluxdigest-stack
  bash install.sh --action rollback --stack-dir /opt/fluxdigest-stack --release-id 20260415070001
  bash install.sh --action status --stack-dir /opt/fluxdigest-stack

Options:
  --action <name>    install | upgrade | rollback | status
  --profile <name>   full | fluxdigest-miniflux | fluxdigest-halo | fluxdigest-only
  --stack-dir <dir>  Stack directory
  --release-id <id>  Release ID for rollback
  --host <value>     Public host or IP for summary output
  --force            Allow overwriting generated files during install/upgrade
  -h, --help         Show this help
EOF
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "缺少必要命令: $1"
  fi
}

require_file_readable() {
  local path="${1:?}"
  [[ -f "${path}" ]] || fail "未找到文件: ${path}"
  [[ -r "${path}" ]] || fail "文件不可读: ${path}"
}

ensure_whiptail() {
  if [[ -x "${WHIPTAIL_BIN}" ]]; then
    return 0
  fi
  command -v "${WHIPTAIL_BIN}" >/dev/null 2>&1 || fail "缺少必要命令: ${WHIPTAIL_BIN}"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --action)
        [[ $# -ge 2 ]] || fail "--action 缺少参数"
        ROOT_ACTION="$2"
        ROOT_NON_INTERACTIVE=1
        shift 2
        ;;
      --profile)
        [[ $# -ge 2 ]] || fail "--profile 缺少参数"
        ROOT_PROFILE="$2"
        ROOT_NON_INTERACTIVE=1
        shift 2
        ;;
      --stack-dir)
        [[ $# -ge 2 ]] || fail "--stack-dir 缺少参数"
        ROOT_STACK_DIR="$2"
        ROOT_NON_INTERACTIVE=1
        shift 2
        ;;
      --release-id)
        [[ $# -ge 2 ]] || fail "--release-id 缺少参数"
        ROOT_RELEASE_ID="$2"
        ROOT_NON_INTERACTIVE=1
        shift 2
        ;;
      --host)
        [[ $# -ge 2 ]] || fail "--host 缺少参数"
        ROOT_HOST="$2"
        ROOT_NON_INTERACTIVE=1
        shift 2
        ;;
      --force)
        ROOT_FORCE=1
        ROOT_NON_INTERACTIVE=1
        shift
        ;;
      -h|--help)
        ROOT_SHOW_HELP=1
        shift
        ;;
      *)
        fail "未知参数: $1"
        ;;
    esac
  done

  case "${ROOT_ACTION}" in
    install|upgrade|rollback|status) ;;
    *) fail "不支持的 action: ${ROOT_ACTION}" ;;
  esac

  if [[ -n "${ROOT_PROFILE}" ]]; then
    case "${ROOT_PROFILE}" in
      full|fluxdigest-miniflux|fluxdigest-halo|fluxdigest-only) ;;
      *) fail "不支持的 profile: ${ROOT_PROFILE}" ;;
    esac
  fi
}

ui_menu() {
  local title="$1"
  local prompt="$2"
  shift 2
  "${WHIPTAIL_BIN}" --title "${title}" --menu "${prompt}" 20 90 10 "$@" 3>&1 1>&2 2>&3
}

ui_input() {
  local title="$1"
  local prompt="$2"
  local initial="$3"
  "${WHIPTAIL_BIN}" --title "${title}" --inputbox "${prompt}" 12 90 "${initial}" 3>&1 1>&2 2>&3
}

ui_confirm() {
  local title="$1"
  local prompt="$2"
  "${WHIPTAIL_BIN}" --title "${title}" --yesno "${prompt}" 12 90
}

ui_message() {
  local title="$1"
  local prompt="$2"
  "${WHIPTAIL_BIN}" --title "${title}" --msgbox "${prompt}" 14 90
}

ui_textbox() {
  local title="$1"
  local file="$2"
  "${WHIPTAIL_BIN}" --title "${title}" --textbox "${file}" 24 100
}

read_env_value() {
  local stack_dir="$1"
  local key="$2"
  local env_file="${stack_dir}/.env"
  [[ -f "${env_file}" ]] || return 0
  sed -n "s/^${key}=//p" "${env_file}" | head -n 1
}

run_stack_action() {
  local action="$1"
  local profile="$2"
  local stack_dir="$3"
  local host="$4"
  local release_id="$5"
  local force_flag="$6"
  local args=(--action "${action}")

  if [[ -n "${profile}" ]]; then
    args+=(--profile "${profile}")
  fi
  args+=(--stack-dir "${stack_dir}")
  if [[ -n "${host}" ]]; then
    args+=(--host "${host}")
  fi
  if [[ -n "${release_id}" ]]; then
    args+=(--release-id "${release_id}")
  fi
  if [[ "${force_flag}" -eq 1 ]]; then
    args+=(--force)
  fi

  bash "${STACK_INSTALL_BIN}" "${args[@]}"
}

main_menu() {
  ui_menu "FluxDigest Installer" "请选择操作" \
    quick "快速安装（推荐）" \
    custom "自定义安装" \
    upgrade "升级现有部署" \
    rollback "回滚到历史版本" \
    status "查看当前部署信息" \
    exit "退出"
}

select_profile() {
  ui_menu "部署组合" "请选择要部署的组件组合" \
    full "FluxDigest + Miniflux + Halo" \
    fluxdigest-miniflux "FluxDigest + Miniflux" \
    fluxdigest-halo "FluxDigest + Halo" \
    fluxdigest-only "仅 FluxDigest"
}

select_release_id() {
  local stack_dir="$1"
  local releases_dir="${stack_dir}/releases"
  [[ -d "${releases_dir}" ]] || fail "未找到 release 目录: ${releases_dir}"

  local items=()
  local release_id
  while IFS= read -r release_id; do
    [[ -n "${release_id}" ]] || continue
    items+=("${release_id}" "历史 release ${release_id}")
  done < <(find "${releases_dir}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | grep -E '^[0-9]{14}$' | sort -r)

  [[ "${#items[@]}" -gt 0 ]] || fail "没有可回滚的历史 release"
  ui_menu "选择回滚版本" "请选择目标 release" "${items[@]}"
}

handle_quick_install() {
  local profile stack_dir host
  profile="$(select_profile)"
  stack_dir="$(ui_input "安装目录" "请输入安装目录" "${DEFAULT_STACK_DIR}")"
  host="$(ui_input "访问地址" "请输入宿主机 IP 或域名" "${DEFAULT_HOST}")"
  ui_confirm "确认安装" "即将部署 ${profile} 到 ${stack_dir}，继续吗？" || return 0
  run_stack_action install "${profile}" "${stack_dir}" "${host}" "" 1
}

handle_custom_install() {
  local profile stack_dir host force_flag=0
  profile="$(select_profile)"
  stack_dir="$(ui_input "安装目录" "请输入安装目录" "${DEFAULT_STACK_DIR}")"
  host="$(ui_input "访问地址" "请输入宿主机 IP 或域名" "${DEFAULT_HOST}")"
  if ui_confirm "覆盖已有文件" "如果目标目录已有生成文件，是否允许覆盖？"; then
    force_flag=1
  fi
  ui_confirm "确认执行" "将以自定义模式部署 ${profile}，继续吗？" || return 0
  run_stack_action install "${profile}" "${stack_dir}" "${host}" "" "${force_flag}"
}

handle_upgrade() {
  local stack_dir host
  stack_dir="$(ui_input "安装目录" "请输入现有安装目录" "${DEFAULT_STACK_DIR}")"
  host="$(read_env_value "${stack_dir}" 'STACK_PUBLIC_HOST')"
  host="$(ui_input "访问地址" "请输入用于展示的宿主机 IP 或域名" "${host:-${DEFAULT_HOST}}")"
  ui_confirm "确认升级" "即将升级 ${stack_dir} 中的现有部署，继续吗？" || return 0
  run_stack_action upgrade "" "${stack_dir}" "${host}" "" 1
}

handle_rollback() {
  local stack_dir release_id
  stack_dir="$(ui_input "安装目录" "请输入现有安装目录" "${DEFAULT_STACK_DIR}")"
  release_id="$(select_release_id "${stack_dir}")"
  ui_confirm "确认回滚" "即将回滚到 ${release_id}，继续吗？" || return 0
  run_stack_action rollback "" "${stack_dir}" "" "${release_id}" 1
}

handle_status() {
  local stack_dir output_file
  stack_dir="$(ui_input "安装目录" "请输入现有安装目录" "${DEFAULT_STACK_DIR}")"
  output_file="$(mktemp)"
  trap 'rm -f "${output_file}"' RETURN
  if run_stack_action status "" "${stack_dir}" "" "" 0 > "${output_file}"; then
    ui_textbox "当前部署信息" "${output_file}"
  else
    ui_message "状态查看失败" "请检查 ${stack_dir} 是否存在且已完成部署。"
    return 1
  fi
}

run_interactive() {
  ensure_whiptail
  while true; do
    case "$(main_menu)" in
      quick)
        handle_quick_install
        return 0
        ;;
      custom)
        handle_custom_install
        return 0
        ;;
      upgrade)
        handle_upgrade
        return 0
        ;;
      rollback)
        handle_rollback
        return 0
        ;;
      status)
        handle_status
        ;;
      exit)
        return 0
        ;;
      *)
        return 0
        ;;
    esac
  done
}

run_non_interactive() {
  local profile="${ROOT_PROFILE}"
  if [[ -z "${profile}" && "${ROOT_ACTION}" == "install" ]]; then
    profile="full"
  fi
  run_stack_action "${ROOT_ACTION}" "${profile}" "${ROOT_STACK_DIR}" "${ROOT_HOST}" "${ROOT_RELEASE_ID}" "${ROOT_FORCE}"
}

main() {
  parse_args "$@"
  if [[ "${ROOT_SHOW_HELP}" -eq 1 ]]; then
    usage
    return 0
  fi

  require_cmd git
  require_cmd curl
  require_file_readable "${STACK_INSTALL_BIN}"

  if [[ "${ROOT_NON_INTERACTIVE}" -eq 1 ]]; then
    run_non_interactive
    return 0
  fi

  run_interactive
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi

