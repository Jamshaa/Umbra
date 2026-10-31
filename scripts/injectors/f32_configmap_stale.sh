#!/usr/bin/env bash
# f32_configmap_stale.sh: inyecta el fallo F32 en el namespace demo.
# Aplica ConfigMap v1 + Deployment y DESPUES sube el ConfigMap a v2 sin
# rollout: los pods se quedan con v1 (falta el checksum/config).
# Re-ejecutar es seguro (el patch a v2 es idempotente).
# Unico binario externo: kubectl (en PATH). Solo ns demo.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create namespace "$NAMESPACE"
fi

kubectl apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f32_configmap_stale.yaml"
kubectl patch configmap f32-settings --namespace "$NAMESPACE" \
  --type merge -p '{"data":{"version":"2"}}'
