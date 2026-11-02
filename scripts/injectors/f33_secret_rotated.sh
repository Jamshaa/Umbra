#!/usr/bin/env bash
# f33_secret_rotated.sh: injects failure F33 into the demo namespace.
# Applies Secret v1 (placeholder) + Deployment and THEN rotates the Secret
# to v2 without rollout: pods mount the old value. Fictitious values,
# no real secret (SECURITY.md). Re-running is safe.
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
  -f "$REPO_ROOT/demo/f33_secret_rotated.yaml"
kubectl --context kind-umbra patch secret f33-rotated --namespace "$NAMESPACE" \
  --type merge -p '{"stringData":{"token":"token-v2"}}'
