#!/usr/bin/env bash
# f37_pdb_blocking.sh (verifier): afirma que el PDB demo F37 fija
# maxUnavailable en 0 (bloquea desalojos y rollouts).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
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
