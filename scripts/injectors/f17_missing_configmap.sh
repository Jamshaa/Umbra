#!/usr/bin/env bash
# f17_missing_configmap.sh: inyecta el fallo F17 en el namespace demo.
# F17 = Referencia a un ConfigMap inexistente (CreateContainerConfigError).
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
  -f "$REPO_ROOT/demo/f17_missing_configmap.yaml"
