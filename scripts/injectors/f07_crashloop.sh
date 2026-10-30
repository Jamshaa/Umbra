#!/usr/bin/env bash
# f07_crashloop.sh: inyecta el fallo F07 en el namespace demo.
# F07 = Contenedor cuyo proceso sale con codigo 1 al arrancar
# (CrashLoopBackOff garantizado, resources y probes sanos).
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
  -f "$REPO_ROOT/demo/f07_crashloop.yaml"
