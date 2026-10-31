#!/usr/bin/env bash
# f23_statefulset_no_headless.sh: inyecta el fallo F23 en el namespace demo.
# F23 = StatefulSet cuyo serviceName no existe (sin headless: sin
# identidades de red estables ni DNS). No crear el Service.
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
  -f "$REPO_ROOT/demo/f23_statefulset_no_headless.yaml"
