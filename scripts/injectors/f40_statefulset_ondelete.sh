#!/usr/bin/env bash
# f40_statefulset_ondelete.sh: inyecta el fallo F40 en el namespace demo.
# F40 = StatefulSet con updateStrategy OnDelete (las nuevas versiones
# nunca se despliegan solas). Incluye su headless Service correcto:
# el fallo es SOLO la estrategia.
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
  -f "$REPO_ROOT/demo/f40_statefulset_ondelete.yaml"
