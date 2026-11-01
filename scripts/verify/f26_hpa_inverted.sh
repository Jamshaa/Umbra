#!/usr/bin/env bash
# f26_hpa_inverted.sh (verifier): afirma que el HPA demo F26 tiene
# minReplicas por encima de maxReplicas (rango imposible).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
HPA="f26-hpa-inverted"

for _ in $(seq 1 30); do
  bounds="$(kubectl --context kind-umbra get hpa "$HPA" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.minReplicas}/{.spec.maxReplicas}' 2>/dev/null || true)"
  min="${bounds%%/*}"
  max="${bounds##*/}"
  if [ -n "$min" ] && [ "$min" -gt "$max" ] 2>/dev/null; then
    echo "F26 verified: HPA minReplicas ($min) above maxReplicas ($max)"
    exit 0
  fi
  sleep 10
done

echo "F26 NOT verified: no inverted HPA in $NAMESPACE/$HPA" >&2
exit 1
