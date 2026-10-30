#!/usr/bin/env bash
# f11_latest_tag.sh: inyecta el fallo F11 en el namespace demo.
# F11 = Imagen fijada al tag mutable :latest.
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
  -f "$REPO_ROOT/demo/f11_latest_tag.yaml"
