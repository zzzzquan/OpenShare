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

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: $command_name" >&2
    exit 1
  fi
}

wait_for_http() {
  local url="$1"
  local label="$2"
  local attempts=30

  for ((i = 1; i <= attempts; i++)); do
    if curl --silent --fail "$url" >/dev/null 2>&1; then
      echo "$label is ready: $url"
      return 0
    fi
    sleep 1
  done

  echo "$label failed to become ready: $url" >&2
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
  echo "reset local test data: $LOCAL_DATA_DIR"
}

prepare_config() {
  mkdir -p "$LOCAL_DATA_DIR" "$LOG_DIR"

  if [[ -f "$BACKEND_CONFIG_LOCAL" ]]; then
    echo "using existing backend config: $BACKEND_CONFIG_LOCAL"
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

  echo "created backend local config: $BACKEND_CONFIG_LOCAL"
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
        echo "initial superadmin credentials:"
        echo "$line"
        return 0
      fi
    fi
    sleep 1
  done

  echo
  echo "initial superadmin password was not detected in time."
  echo "check backend log manually: $BACKEND_LOG"
  return 1
}

print_summary() {
  echo
  echo "OpenShare dev test environment is running."
  echo "public page: http://localhost:5173/"
  echo "admin page:  http://localhost:5173/admin"
  echo "backend log: $BACKEND_LOG"
  echo "frontend log: $FRONTEND_LOG"
  if [[ -n "$IMPORT_ROOT" ]]; then
    echo "local import test directory: $IMPORT_ROOT"
  fi
  echo
  echo "this script resets the local database and storage on every startup for clean testing."
  echo "press Ctrl+C to stop both services."
}

main() {
  require_command go
  require_command npm
  require_command curl

  trap cleanup EXIT INT TERM

  reset_local_state
  prepare_config
  start_backend
  start_frontend

  wait_for_http "http://127.0.0.1:8080/api/public/files" "backend"
  print_initial_admin_credentials
  wait_for_http "http://127.0.0.1:5173/" "frontend"
  print_summary

  wait
}

main "$@"
