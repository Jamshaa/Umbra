#!/usr/bin/env bash
# f25_no_grace_period.sh (verifier): asserts that demo pod F25 has
# terminationGracePeriodSeconds 0 (death without draining).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f25-no-grace-period"

for _ in $(seq 1 30); do
  grace="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.terminationGracePeriodSeconds}' 2>/dev/null || true)"
  if [ "$grace" = "0" ]; then
    echo "F25 verified: termination grace period is zero (abrupt kills)"
    exit 0
  fi
  sleep 10
done

echo "F25 NOT verified: grace period not zero in $NAMESPACE/$DEPLOY" >&2
exit 1
