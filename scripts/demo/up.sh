#!/usr/bin/env bash
# up.sh: brings up the full Umbra environment in kind (flow step 6).
# Cluster with audit log -> images -> CRDs -> manager -> base namespaces.
# Idempotent: re-running rebuilds from scratch.
# Does NOT apply the 40 fixtures: each injector applies its failure in turn.
# External binaries: kind (KIND_BIN or PATH), kubectl, docker, go.
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT_DIR"

CTX="kind-umbra"

bash scripts/kind/create-cluster.sh            # creates kind-umbra with audit
source scripts/lib/guard.sh                   # guard after creating the cluster
kubectl --context "$CTX" wait --for=condition=Ready node --all --timeout=180s
bash scripts/build-images.sh                  # builds and loads images into kind
kubectl --context "$CTX" apply --server-side -k config/crd
kubectl --context "$CTX" apply -k config/manager
kubectl --context "$CTX" -n umbra-system rollout status deploy/umbra-manager --timeout=120s
kubectl --context "$CTX" create ns demo --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -
kubectl --context "$CTX" create ns umbra-sandbox --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -
kubectl --context "$CTX" apply -f config/sandbox/deny-all.yaml

echo "demo lista: kind-umbra Ready, CRDs + manager en marcha"
