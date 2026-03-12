#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
LOCAL_DATA_DIR="$ROOT_DIR/.localdata"
LOG_DIR="$LOCAL_DATA_DIR/logs"
BACKEND_LOG="$LOG_DIR/backend.log"
FRONTEND_LOG="$LOG_DIR/frontend.log"
BACKEND_CONFIG_LOCAL="$BACKEND_DIR/configs/config.local.json"
IMPORT_ROOT="${1:-}"

BACKEND_PID=""
FRONTEND_PID=""
TOTAL_STEPS=6

print_step() {
  local step="$1"
  local message="$2"
  echo
  echo "[$step/$TOTAL_STEPS] $message"
}

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: $command_name" >&2
    exit 1
  fi
}

ensure_port_available() {
  local port="$1"
  local label="$2"
  local occupancy

  occupancy="$(lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -z "$occupancy" ]]; then
    return
  fi

  echo "$label 端口 $port 已被占用，当前无法安全启动测试环境。"
  echo "请先停止占用该端口的旧进程："
  echo "$occupancy"
  exit 1
}

prepare_frontend_dependencies() {
  if [[ -x "$FRONTEND_DIR/node_modules/.bin/vite" ]]; then
    echo "前端依赖已存在，跳过安装。"
    return
  fi

  echo "前端依赖缺失，正在执行 npm install ..."
  (
    cd "$FRONTEND_DIR"
    npm install
  )
}

wait_for_http() {
  local url="$1"
  local label="$2"
  local attempts=30

  for ((i = 1; i <= attempts; i++)); do
    if curl --silent --fail "$url" >/dev/null 2>&1; then
      echo "$label 已就绪：$url"
      return 0
    fi
    sleep 1
  done

  echo "$label 启动失败：$url" >&2
  return 1
}

cleanup() {
  local exit_code=$?

  if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
    kill "$BACKEND_PID" >/dev/null 2>&1 || true
  fi

  if [[ -n "$FRONTEND_PID" ]] && kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
    kill "$FRONTEND_PID" >/dev/null 2>&1 || true
  fi

  exit "$exit_code"
}

reset_local_state() {
  rm -rf "$LOCAL_DATA_DIR"
  echo "已重置本地测试数据：$LOCAL_DATA_DIR"
}

prepare_config() {
  mkdir -p "$LOCAL_DATA_DIR" "$LOG_DIR"

  if [[ -f "$BACKEND_CONFIG_LOCAL" ]]; then
    echo "检测到已有后端本地配置：$BACKEND_CONFIG_LOCAL"
    return
  fi

  cat >"$BACKEND_CONFIG_LOCAL" <<EOF
{
  "database": {
    "path": "$LOCAL_DATA_DIR/openshare.db"
  },
  "storage": {
    "root": "$LOCAL_DATA_DIR"
  },
  "session": {
    "secret": "dev-local-session-secret"
  }
}
EOF

  echo "已创建后端本地配置：$BACKEND_CONFIG_LOCAL"
}

start_backend() {
  (
    cd "$BACKEND_DIR"
    go run ./cmd/server >"$BACKEND_LOG" 2>&1
  ) &
  BACKEND_PID=$!
}

start_frontend() {
  (
    cd "$FRONTEND_DIR"
    npm run dev -- --host 0.0.0.0 >"$FRONTEND_LOG" 2>&1
  ) &
  FRONTEND_PID=$!
}

print_initial_admin_credentials() {
  local attempts=30
  local line=""

  for ((i = 1; i <= attempts; i++)); do
    if [[ -f "$BACKEND_LOG" ]]; then
      line="$(grep -E '\[bootstrap\] super admin initialized; username=.* password=.*' "$BACKEND_LOG" | tail -n 1 || true)"
      if [[ -n "$line" ]]; then
        echo
        echo "超级管理员初始凭据："
        echo "$line"
        return 0
      fi
    fi
    sleep 1
  done

  echo
  echo "暂未在限定时间内检测到超级管理员初始密码。"
  echo "请手动查看后端日志：$BACKEND_LOG"
  return 1
}

print_summary() {
  echo
  echo "=================================="
  echo "OpenShare 本地测试环境已就绪"
  echo "=================================="
  echo
  echo "浏览器入口："
  echo "  公开页   : http://localhost:5173/"
  echo "  管理页   : http://localhost:5173/admin"
  echo
  echo "运行日志："
  echo "  后端日志 : $BACKEND_LOG"
  echo "  前端日志 : $FRONTEND_LOG"
  if [[ -n "$IMPORT_ROOT" ]]; then
    echo "  导入目录 : $IMPORT_ROOT"
  fi
  echo
  echo "当前脚本每次启动都会清空本地数据库和存储目录，适合删档测试。"
  echo "按 Ctrl+C 可同时停止前后端服务。"
}

main() {
  require_command go
  require_command npm
  require_command curl
  require_command lsof

  trap cleanup EXIT INT TERM

  ensure_port_available 8080 "后端"
  ensure_port_available 5173 "前端"

  print_step 1 "重置本地测试数据"
  reset_local_state

  print_step 2 "准备后端本地配置"
  prepare_config

  print_step 3 "检查前端依赖"
  prepare_frontend_dependencies

  print_step 4 "启动后端服务"
  start_backend
  wait_for_http "http://127.0.0.1:8080/api/public/files" "后端"
  print_initial_admin_credentials

  print_step 5 "启动前端服务"
  start_frontend
  wait_for_http "http://127.0.0.1:5173/" "前端"

  print_step 6 "输出测试入口与推荐流程"
  print_summary

  wait
}

main "$@"
