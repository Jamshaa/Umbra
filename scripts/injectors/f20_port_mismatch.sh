#!/usr/bin/env bash
# f20_port_mismatch.sh: inyecta el fallo F20 en el namespace demo.
# F20 = Probes contra el puerto 9090 cuando el contenedor abre 8080.
# Resources sanos para aislar el fallo.
# Idempotente: re-ejecutar es seguro (kubectl --context kind-umbra apply).
# Unico binario externo: kubectl (en PATH). No toca otros namespaces.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl --context kind-umbra get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl --context kind-umbra create namespace "$NAMESPACE"
fi

kubectl --context kind-umbra apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f20_port_mismatch.yaml"
