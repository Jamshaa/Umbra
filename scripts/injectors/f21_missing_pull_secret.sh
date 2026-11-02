#!/usr/bin/env bash
# f21_missing_pull_secret.sh: injects failure F21 into the demo namespace.
# F21 = Imagen de registro privado sin imagePullSecrets (el kubelet no
# fails to authenticate: permanent ImagePullBackOff).
# Healthy resources and probes to isolate the failure.
# Idempotent: re-running is safe (kubectl --context kind-umbra apply).
# Only external binary: kubectl (in PATH). Touches no other namespaces.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl --context kind-umbra get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl --context kind-umbra create namespace "$NAMESPACE"
fi

kubectl --context kind-umbra apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f21_missing_pull_secret.yaml"
