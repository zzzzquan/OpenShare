#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_NAME="openshare-linux-builder"
DOCKERFILE_PATH="$ROOT_DIR/docker/build-linux.Dockerfile"

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: $command_name" >&2
    exit 1
  fi
}

build_image() {
  echo "building docker image ..."
  docker build --platform linux/amd64 -t "$IMAGE_NAME" -f "$DOCKERFILE_PATH" "$ROOT_DIR"
}

run_build() {
  echo "running linux build in docker ..."
  docker run --rm \
    --platform linux/amd64 \
    --user "$(id -u):$(id -g)" \
    -e HOME=/tmp/openshare-build-home \
    -e npm_config_cache=/tmp/openshare-build-home/.npm \
    -v "$ROOT_DIR:/workspace" \
    -w /workspace \
    "$IMAGE_NAME" \
    bash -c 'rm -rf ./frontend/node_modules && bash ./scripts/build-linux.sh'
}

main() {
  require_command docker
  build_image
  run_build
}

main "$@"
