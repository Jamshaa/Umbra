#!/usr/bin/env bash
# f08_readiness_mismatch.sh: inyecta el fallo F08 en el namespace demo.
# F08 = readinessProbe contra un puerto donde nada escucha (8081 vs
# 8080 real): pod vivo pero nunca Ready. Liveness y resources sanos.
# Idempotente: re-ejecutar es seguro (kubectl --context kind-umbra apply).
# Unico binario externo: kubectl (en PATH). No toca otros namespaces.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl --context kind-umbra get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl --context kind-umbra create namespace "$NAMESPACE"
fi

kubectl --context kind-umbra apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f08_readiness_mismatch.yaml"
