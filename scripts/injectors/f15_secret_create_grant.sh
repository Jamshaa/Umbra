#!/usr/bin/env bash
# f15_secret_create_grant.sh: inyecta el fallo F15 en el namespace demo.
# F15 = Role que concede create sobre secrets a la SA default
# (permiso de escritura innecesario sobre material sensible).
# Idempotente: re-ejecutar es seguro (kubectl --context kind-umbra apply).
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
  -f "$REPO_ROOT/demo/f15_secret_create_grant.yaml"
