#!/usr/bin/env bash
# f06_no_readiness.sh: inyecta el fallo F06 en el namespace demo.
# F06 = Contenedor SIN readiness probe (con limits sanos y liveness
# definida): el pod arranca pero nunca esta Ready.
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
  -f "$REPO_ROOT/demo/f06_no_readiness.yaml"
