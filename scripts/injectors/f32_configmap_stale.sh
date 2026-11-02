#!/usr/bin/env bash
# f32_configmap_stale.sh: injects failure F32 into the demo namespace.
# Applies ConfigMap v1 + Deployment and THEN bumps the ConfigMap to v2 without
# rollout: pods stay on v1 (missing checksum/config).
# Re-running is safe (the patch to v2 is idempotent).
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
  -f "$REPO_ROOT/demo/f32_configmap_stale.yaml"
kubectl --context kind-umbra patch configmap f32-settings --namespace "$NAMESPACE" \
  --type merge -p '{"data":{"version":"2"}}'
