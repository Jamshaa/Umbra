#!/usr/bin/env bash
# up.sh: levanta el entorno completo de Umbra en kind (paso 6 del flujo).
# Cluster con audit log -> imagenes -> CRDs -> manager -> namespaces base.
# Idempotente: re-ejecutar reconstruye desde cero.
# NO aplica los 40 fixtures: cada inyector aplica su fallo cuando toca.
# Binarios externos: kind (KIND_BIN o PATH), kubectl, docker, go.
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT_DIR"

CTX="kind-umbra"

bash scripts/kind/create-cluster.sh            # crea kind-umbra con audit
source scripts/lib/guard.sh                   # guarda tras crear el cluster
kubectl --context "$CTX" wait --for=condition=Ready node --all --timeout=180s
bash scripts/build-images.sh                  # compila y carga imagenes en kind
kubectl --context "$CTX" apply --server-side -k config/crd
kubectl --context "$CTX" apply -k config/manager
kubectl --context "$CTX" -n umbra-system rollout status deploy/umbra-manager --timeout=120s
kubectl --context "$CTX" create ns demo --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -
kubectl --context "$CTX" create ns umbra-sandbox --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -
kubectl --context "$CTX" apply -f config/sandbox/deny-all.yaml

echo "demo lista: kind-umbra Ready, CRDs + manager en marcha"
