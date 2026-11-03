#!/usr/bin/env bash
# audit-export.sh: copy the kind audit log out of the control-plane node
# into .demo/audit.log, filtered to the demo namespace. The log lives
# inside the node (see docs/kind-audit.md), so this goes through docker.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
mkdir -p .demo
RAW="$(mktemp)"
trap 'rm -f "$RAW"' EXIT
MSYS_NO_PATHCONV=1 docker exec umbra-control-plane cat /var/log/umbra-audit/audit.log >"$RAW"
grep '"namespace":"demo"' "$RAW" >.demo/audit.log || true
echo "audit log: $(wc -l <.demo/audit.log) demo lines -> .demo/audit.log"
