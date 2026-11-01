#!/usr/bin/env bash
# f13_cluster_admin_binding.sh (verifier): afirma que existe el
# ClusterRoleBinding f13 que ata la SA demo a cluster-admin.
# Solo kubectl. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

for _ in $(seq 1 30); do
  role="$(kubectl --context kind-umbra get clusterrolebinding f13-overprivileged-binding \
    -o 'jsonpath={.roleRef.name}/{.subjects[*].name}' 2>/dev/null || true)"
  if [ "$role" = "cluster-admin/f13-overprivileged" ]; then
    echo "F13 verified: demo ServiceAccount bound to cluster-admin"
    exit 0
  fi
  sleep 10
done

echo "F13 NOT verified: no f13-overprivileged-binding to cluster-admin" >&2
exit 1
