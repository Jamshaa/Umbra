#!/usr/bin/env bash
# f12_allow_priv_esc.sh: inyecta el fallo F12 en el namespace demo.
# F12 = allowPrivilegeEscalation=true + capacidad SYS_ADMIN.
# Resources y probes sanos para aislar el fallo.
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
  -f "$REPO_ROOT/demo/f12_allow_priv_esc.yaml"
