#!/usr/bin/env bash
# f37_pdb_blocking.sh (verifier): asserts that demo PDB F37 sets
# maxUnavailable at 0 (blocks evictions and rollouts).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
PDB="f37-blocking"

for _ in $(seq 1 30); do
  mu="$(kubectl --context kind-umbra get pdb "$PDB" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.maxUnavailable}' 2>/dev/null || true)"
  if [ "$mu" = "0" ]; then
    echo "F37 verified: PDB blocks all voluntary evictions"
    exit 0
  fi
  sleep 10
done

echo "F37 NOT verified: no blocking PDB in $NAMESPACE/$PDB" >&2
exit 1
