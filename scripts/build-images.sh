#!/usr/bin/env bash
# build-images.sh: static linux binaries + scratch images, loaded into kind.
# Offline-safe: FROM scratch pulls nothing; Go builds use the module cache.
# Images: umbra-manager:dev, umbra-api:dev, demo-app:local (all kind-only).
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

KIND_BIN="${KIND_BIN:-$HOME/go/bin/kind}"

mkdir -p .build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o .build/umbra-manager ./cmd
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o .build/umbra-api ./cmd/umbra-api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o .build/demo-server ./demo/server

docker build -f Dockerfile.manager -t umbra-manager:dev .
docker build -f Dockerfile.umbra-api -t umbra-api:dev .
docker build -f Dockerfile.demo -t demo-app:local .

if [ "${KIND_LOAD_SKIP:-0}" = "1" ]; then
  echo "KIND_LOAD_SKIP=1: images built, skipping kind load"
  exit 0
fi
"$KIND_BIN" load docker-image umbra-manager:dev umbra-api:dev demo-app:local --name umbra
