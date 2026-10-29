#!/usr/bin/env bash
# create-cluster.sh: recrea el cluster kind `umbra` (contexto kind-umbra)
# con el audit log del API server habilitado desde el arranque.
#
# - La audit policy es scripts/kind/audit-policy.yaml (restrictiva: solo
#   create/update/patch sobre workloads en namespaces de demo).
# - El audit log vive DENTRO del nodo en /var/log/umbra-audit/audit.log.
# - Idempotente: se puede re-ejecutar; borra el cluster anterior si existe.
# - Unicos binarios externos: kind (KIND_BIN o PATH) y kubectl.
set -eu

CLUSTER_NAME="umbra"
KIND_IMAGE="kindest/node:v1.37.0"
AUDIT_LOG_PATH="/var/log/umbra-audit/audit.log"
AUDIT_LOG_DIR="$(dirname "$AUDIT_LOG_PATH")"
AUDIT_POLICY_NODE="/etc/kubernetes/umbra-audit-policy.yaml"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
AUDIT_POLICY_HOST="$ROOT_DIR/scripts/kind/audit-policy.yaml"

# kind rara vez esta en el PATH (ver AGENTS.md §3): se admite override
# via KIND_BIN y fallback a la ruta documentada en Windows.
KIND_BIN="${KIND_BIN:-$(command -v kind 2>/dev/null || echo "$HOME/go/bin/kind")}"

"$KIND_BIN" delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true

KIND_CONFIG="$(mktemp)"
trap 'rm -f "$KIND_CONFIG"' EXIT INT TERM

cat >"$KIND_CONFIG" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        audit-log-path: "$AUDIT_LOG_PATH"
        audit-policy-file: "$AUDIT_POLICY_NODE"
        audit-log-maxage: "7"
        audit-log-maxbackup: "3"
        audit-log-maxsize: "100"
      extraVolumes:
      - name: umbra-audit-log
        hostPath: "$AUDIT_LOG_DIR"
        mountPath: "$AUDIT_LOG_DIR"
        pathType: DirectoryOrCreate
      - name: umbra-audit-policy
        hostPath: "$AUDIT_POLICY_NODE"
        mountPath: "$AUDIT_POLICY_NODE"
        readOnly: true
        pathType: File
  extraMounts:
  - hostPath: "$AUDIT_POLICY_HOST"
    containerPath: "$AUDIT_POLICY_NODE"
    readOnly: true
EOF

"$KIND_BIN" create cluster --name "$CLUSTER_NAME" --image "$KIND_IMAGE" --config "$KIND_CONFIG"

kubectl wait --for=condition=Ready nodes --all --timeout=120s
kubectl cluster-info --context "kind-$CLUSTER_NAME"
