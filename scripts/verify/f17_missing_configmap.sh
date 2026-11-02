#!/usr/bin/env bash
# f17_missing_configmap.sh (verifier): asserts that demo pod F17 is
# stuck in CreateContainerConfigError over the missing ConfigMap.
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f17-missing-configmap"

for _ in $(seq 1 30); do
  waiting="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || true)"
  if printf '%s' "$waiting" | grep -q 'CreateContainerConfigError'; then
    echo "F17 verified: pod stuck on missing ConfigMap"
    exit 0
  fi
  sleep 10
done

echo "F17 NOT verified: no config error in $NAMESPACE/$LABEL" >&2
exit 1
