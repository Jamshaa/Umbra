#!/usr/bin/env bash
# f36_service_no_endpoints.sh (verifier): afirma que el Service demo F36
# existe pero no tiene endpoints (selector sin pods).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
SVC="f36-no-endpoints"

for _ in $(seq 1 30); do
  svc="$(kubectl --context kind-umbra get service "$SVC" --namespace "$NAMESPACE" \
    -o 'jsonpath={.metadata.name}' 2>/dev/null || true)"
  eps="$(kubectl --context kind-umbra get endpoints "$SVC" --namespace "$NAMESPACE" \
    -o 'jsonpath={.subsets}' 2>/dev/null || true)"
  if [ "$svc" = "$SVC" ] && [ -z "$eps" ]; then
    echo "F36 verified: Service exists with no endpoints"
    exit 0
  fi
  sleep 10
done

echo "F36 NOT verified: Service has endpoints or is missing in $NAMESPACE/$SVC" >&2
exit 1
