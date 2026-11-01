#!/usr/bin/env bash
# f23_statefulset_no_headless.sh (verifier): afirma que el Service
# f23-missing-headless NO existe (el StatefulSet no tiene headless).
# La ausencia es el estado estable: bastan 3 sondeos.
# Solo kubectl, solo ns demo.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"

for _ in $(seq 1 3); do
  if ! kubectl --context kind-umbra get service f23-missing-headless --namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo "F23 verified: StatefulSet without headless Service"
    exit 0
  fi
  sleep 10
done

echo "F23 NOT verified: headless Service exists in $NAMESPACE" >&2
exit 1
