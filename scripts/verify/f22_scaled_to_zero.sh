#!/usr/bin/env bash
# f22_scaled_to_zero.sh (verifier): afirma que el Deployment demo F22
# esta escalado a cero replicas (existe pero no sirve trafico).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f22-scaled-to-zero"

for _ in $(seq 1 30); do
  reps="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.replicas}' 2>/dev/null || true)"
  if [ "$reps" = "0" ]; then
    echo "F22 verified: Deployment scaled to zero replicas"
    exit 0
  fi
  sleep 10
done

echo "F22 NOT verified: replicas != 0 in $NAMESPACE/$DEPLOY" >&2
exit 1
