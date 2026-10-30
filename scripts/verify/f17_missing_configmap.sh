#!/usr/bin/env bash
# f17_missing_configmap.sh (verifier): afirma que el pod demo F17 esta
# atascado en CreateContainerConfigError por el ConfigMap ausente.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f17-missing-configmap"

for _ in $(seq 1 30); do
  waiting="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || true)"
  if printf '%s' "$waiting" | grep -q 'CreateContainerConfigError'; then
    echo "F17 verified: pod stuck on missing ConfigMap"
    exit 0
  fi
  sleep 10
done

echo "F17 NOT verified: no config error in $NAMESPACE/$LABEL" >&2
exit 1
