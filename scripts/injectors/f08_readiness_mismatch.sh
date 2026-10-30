#!/usr/bin/env bash
# f08_readiness_mismatch.sh: inyecta el fallo F08 en el namespace demo.
# F08 = readinessProbe contra un puerto donde nada escucha (8081 vs
# 8080 real): pod vivo pero nunca Ready. Liveness y resources sanos.
# Idempotente: re-ejecutar es seguro (kubectl apply).
# Unico binario externo: kubectl (en PATH). No toca otros namespaces.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create namespace "$NAMESPACE"
fi

kubectl apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f08_readiness_mismatch.yaml"
