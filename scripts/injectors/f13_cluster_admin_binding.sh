#!/usr/bin/env bash
# f13_cluster_admin_binding.sh: inyecta el fallo F13 en el namespace demo.
# F13 = ServiceAccount demo atada a cluster-admin (poder total).
# Idempotente: re-ejecutar es seguro (kubectl apply).
# Unico binario externo: kubectl (en PATH). Solo crea la SA en demo;
# el ClusterRoleBinding es cluster-scoped por definicion (es el fallo).
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="demo"

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create namespace "$NAMESPACE"
fi

kubectl apply --namespace "$NAMESPACE" \
  -f "$REPO_ROOT/demo/f13_cluster_admin_binding.yaml"
