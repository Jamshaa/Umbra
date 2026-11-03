#!/usr/bin/env bash
# build-bins.sh: host binaries for the demo (no go run on camera).
# Builds umbra, umbra-agent, umbra-bench and umbra-api for the host OS
# into .build/host/ (.exe suffix on Windows). Idempotent. Never touches
# the cluster, so no guard.sh / kubectl context is needed here.
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p .build/host

EXT=""
case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN* | Windows_NT) EXT=".exe" ;;
esac

for bin in umbra umbra-agent umbra-bench umbra-api; do
	go build -o ".build/host/${bin}${EXT}" "./cmd/${bin}"
	echo "built .build/host/${bin}${EXT}"
done
