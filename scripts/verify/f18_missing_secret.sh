#!/usr/bin/env bash
# f18_missing_secret.sh (verifier): afirma que el pod demo F18 esta
# atascado en CreateContainerConfigError por el Secret ausente.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f18-missing-secret"

for _ in $(seq 1 30); do
  waiting="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || true)"
  if printf '%s' "$waiting" | grep -q 'CreateContainerConfigError'; then
    echo "F18 verified: pod stuck on missing Secret"
    exit 0
  fi
  sleep 10
done

echo "F18 NOT verified: no secret error in $NAMESPACE/$LABEL" >&2
exit 1
