#!/usr/bin/env bash
# f13_cluster_admin_binding.sh: injects failure F13 into the demo namespace.
# F13 = Demo ServiceAccount bound to cluster-admin (total power).
# Idempotent: re-running is safe (kubectl --context kind-umbra apply).
# Only external binary: kubectl (in PATH). Only creates the SA in demo;
# the ClusterRoleBinding is cluster-scoped by definition (that is the failure).
set -eu
source "$(dirname "$0")/../lib/guard.sh"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl --context kind-umbra get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl --context kind-umbra create namespace "$NAMESPACE"
fi

kubectl --context kind-umbra apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f13_cluster_admin_binding.yaml"
