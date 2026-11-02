#!/usr/bin/env bash
# f14_wildcard_role.sh (verifier): asserts that demo Role F14 grants
# "*" in resources and verbs. kubectl only, demo ns only.
# Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
ROLENAME="f14-wildcard"

for _ in $(seq 1 30); do
  # Both lists are exactly ["*"] as JSON.
  res="$(kubectl --context kind-umbra get role "$ROLENAME" --namespace "$NAMESPACE" \
    -o 'jsonpath={.rules[0].resources}{.rules[0].verbs}' 2>/dev/null || true)"
  if [ "$res" = '["*"]["*"]' ]; then
    echo "F14 verified: Role grants wildcard resources and verbs"
    exit 0
  fi
  sleep 10
done

echo "F14 NOT verified: no wildcard Role in $NAMESPACE/$ROLENAME" >&2
exit 1
