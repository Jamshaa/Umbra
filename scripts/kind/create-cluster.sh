#!/usr/bin/env bash
# create-cluster.sh: recreates the kind cluster `umbra` (kind-umbra context)
# with the API server audit log enabled from boot.
#
# - The audit policy is scripts/kind/audit-policy.yaml (restrictive: only
# create/update/patch on workloads in demo namespaces).
# - The audit log lives INSIDE the node at /var/log/umbra-audit/audit.log.
# - Idempotent: can be re-run; deletes the previous cluster if one exists.
# - Only external binaries: kind (KIND_BIN or PATH) and kubectl.
set -eu

CLUSTER_NAME="umbra"
KIND_IMAGE="kindest/node:v1.37.0"
AUDIT_LOG_PATH="/var/log/umbra-audit/audit.log"
AUDIT_LOG_DIR="$(dirname "$AUDIT_LOG_PATH")"
AUDIT_POLICY_NODE="/etc/kubernetes/umbra-audit-policy.yaml"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
AUDIT_POLICY_HOST="$ROOT_DIR/scripts/kind/audit-policy.yaml"

# kind is rarely on PATH (see AGENTS.md §3): override accepted
# via KIND_BIN with fallback to the documented path on Windows.
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

# From here on we talk to the fresh cluster: enforce the context guard
# for every kubectl call below (the delete above is name-scoped by kind).
source "$(dirname "$0")/../lib/guard.sh"

kubectl --context kind-umbra wait --for=condition=Ready nodes --all --timeout=120s
kubectl --context kind-umbra cluster-info
