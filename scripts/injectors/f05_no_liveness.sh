#!/usr/bin/env bash
# f05_no_liveness.sh: inyecta el fallo F05 en el namespace demo.
# F05 = Contenedor SIN liveness probe (con limits sanos y readiness
# definida): si la app se cuelga, nada la reinicia.
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
  -f "$REPO_ROOT/demo/f05_no_liveness.yaml"
