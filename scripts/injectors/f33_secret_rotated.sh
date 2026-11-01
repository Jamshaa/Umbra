#!/usr/bin/env bash
# f33_secret_rotated.sh: inyecta el fallo F33 en el namespace demo.
# Aplica Secret v1 (placeholder) + Deployment y DESPUES rota el Secret
# a v2 sin rollout: los pods montan el valor antiguo. Valores ficticios,
# ningun secreto real (SECURITY.md). Re-ejecutar es seguro.
# Unico binario externo: kubectl (en PATH). Solo ns demo.
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
