#!/usr/bin/env bash
# f31_pods_exec_grant.sh: injects failure F31 into the demo namespace.
# F31 = Role with create on pods/exec (shell in any pod).
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
  -f "$REPO_ROOT/demo/f31_pods_exec_grant.yaml"
