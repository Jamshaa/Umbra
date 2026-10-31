#!/usr/bin/env bash
# f26_hpa_inverted.sh: inyecta el fallo F26 en el namespace demo.
# F26 = HPA con minReplicas por encima de maxReplicas (nunca escala).
# Idempotente: re-ejecutar es seguro (kubectl apply).
# Unico binario externo: kubectl (en PATH). Solo ns demo.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create namespace "$NAMESPACE"
fi

kubectl apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f26_hpa_inverted.yaml"
