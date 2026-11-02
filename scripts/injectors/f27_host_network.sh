#!/usr/bin/env bash
# f27_host_network.sh: injects failure F27 into the demo namespace.
# F27 = Pod with hostNetwork (sees the node network).
# Idempotent: re-running is safe (kubectl --context kind-umbra apply).
# Only external binary: kubectl (in PATH). Demo ns only.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl --context kind-umbra get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl --context kind-umbra create namespace "$NAMESPACE"
fi

kubectl --context kind-umbra apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f27_host_network.yaml"
