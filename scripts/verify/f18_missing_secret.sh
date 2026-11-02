#!/usr/bin/env bash
# f18_missing_secret.sh (verifier): asserts that demo pod F18 is
# stuck in CreateContainerConfigError over the missing Secret.
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f18-missing-secret"

for _ in $(seq 1 30); do
  waiting="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || true)"
  if printf '%s' "$waiting" | grep -q 'CreateContainerConfigError'; then
    echo "F18 verified: pod stuck on missing Secret"
    exit 0
  fi
  sleep 10
done

echo "F18 NOT verified: no secret error in $NAMESPACE/$LABEL" >&2
exit 1
