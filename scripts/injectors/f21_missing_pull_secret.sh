#!/usr/bin/env bash
# f21_missing_pull_secret.sh: inyecta el fallo F21 en el namespace demo.
# F21 = Imagen de registro privado sin imagePullSecrets (el kubelet no
# puede autenticarse: ImagePullBackOff permanente).
# Resources y probes sanos para aislar el fallo.
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
  -f "$REPO_ROOT/demo/f21_missing_pull_secret.yaml"
