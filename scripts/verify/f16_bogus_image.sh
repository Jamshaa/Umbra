#!/usr/bin/env bash
# f16_bogus_image.sh (verifier): afirma que el pod demo F16 esta
# atascado en ImagePullBackOff o ErrImagePull.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f16-bogus-image"

for _ in $(seq 1 30); do
  waiting="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || true)"
  # Cualquiera de los dos motivos confirma imagen inexistente.
  if printf '%s' "$waiting" | grep -Eq 'ImagePullBackOff|ErrImagePull'; then
    echo "F16 verified: pod stuck pulling bogus image ($waiting)"
    exit 0
  fi
  sleep 10
done

echo "F16 NOT verified: no image-pull failure in $NAMESPACE/$LABEL" >&2
exit 1
