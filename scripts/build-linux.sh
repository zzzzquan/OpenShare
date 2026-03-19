#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
BACKEND_DIR="$ROOT_DIR/backend"
EMBED_DIR="$BACKEND_DIR/web/dist"
RELEASE_DIR="$ROOT_DIR/release/amd64"
OUTPUT_BIN="$RELEASE_DIR/openshare"

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: $command_name" >&2
    exit 1
  fi
}

prepare_frontend() {
  if [[ -x "$FRONTEND_DIR/node_modules/.bin/vite" ]] && frontend_tooling_healthy; then
    :
  else
    echo "frontend dependencies missing or broken, running npm install ..."
    (
      cd "$FRONTEND_DIR"
      npm install
    )

    if ! frontend_tooling_healthy; then
      echo "frontend dependencies still incomplete, reinstalling from a clean node_modules ..."
      rm -rf "$FRONTEND_DIR/node_modules"
      (
        cd "$FRONTEND_DIR"
        npm install
      )
    fi
  fi

  echo "building frontend ..."
  (
    cd "$FRONTEND_DIR"
    npm run build
  )
}

frontend_tooling_healthy() {
  (
    cd "$FRONTEND_DIR"
    node -e "require('vite'); require('rollup');"
  ) >/dev/null 2>&1
}

sync_frontend_dist() {
  echo "syncing frontend dist into backend embed directory ..."
  rm -rf "$EMBED_DIR"
  mkdir -p "$EMBED_DIR"
  cp -R "$FRONTEND_DIR/dist/." "$EMBED_DIR/"
}

build_backend() {
  echo "building linux binary ..."
  mkdir -p "$RELEASE_DIR"
  (
    cd "$BACKEND_DIR"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o "$OUTPUT_BIN" ./cmd/server
  )
}

package_release() {
  echo "packaging release directory ..."
  mkdir -p "$RELEASE_DIR/configs" "$RELEASE_DIR/data/staging" "$RELEASE_DIR/data/trash"
  cp "$BACKEND_DIR/configs/config.default.json" "$RELEASE_DIR/configs/config.default.json"
  cat >"$RELEASE_DIR/configs/config.local.json" <<'EOF'
{
  "server": {
    "host": "0.0.0.0",
    "port": 8890
  },
  "database": {
    "path": "data/openshare.db"
  },
  "storage": {
    "root": "data"
  },
  "session": {
    "secret": "change-this-session-secret-before-production"
  }
}
EOF
  cat >"$RELEASE_DIR/start.sh" <<'EOF'
#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

exec ./openshare
EOF
  chmod +x "$RELEASE_DIR/start.sh"
}

print_summary() {
  echo
  echo "build complete"
  echo "binary: $OUTPUT_BIN"
  echo "embedded frontend source: $EMBED_DIR"
}

main() {
  require_command go
  require_command npm
  require_command cp
  require_command rm

  prepare_frontend
  sync_frontend_dist
  build_backend
  package_release
  print_summary
}

main "$@"
