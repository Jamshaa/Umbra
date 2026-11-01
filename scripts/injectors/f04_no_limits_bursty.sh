#!/usr/bin/env bash
# f04_no_limits_bursty.sh: inyecta el fallo F04 en el namespace demo.
# F04 = Deployment SIN resources (ni limits ni requests) con carga
# de memoria a rafagas: sin techo, el consumo es ilimitado y los
# reinicios escalan bajo carga.
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
  -f "$REPO_ROOT/demo/f04_no_limits_bursty.yaml"
