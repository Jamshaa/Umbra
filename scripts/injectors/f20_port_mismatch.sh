#!/usr/bin/env bash
# f20_port_mismatch.sh: injects failure F20 into the demo namespace.
# F20 = Probes against port 9090 while the container opens 8080.
# Healthy resources to isolate the failure.
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
  -f "$REPO_ROOT/demo/f20_port_mismatch.yaml"
