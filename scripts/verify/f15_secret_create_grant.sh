#!/usr/bin/env bash
# f15_secret_create_grant.sh (verifier): asserts that demo Role F15
# grants create on secrets. kubectl only, demo ns only.
# Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
ROLENAME="f15-secret-create"

for _ in $(seq 1 30); do
  # resources == [secrets] and verbs include create.
  res="$(kubectl --context kind-umbra get role "$ROLENAME" --namespace "$NAMESPACE" \
    -o 'jsonpath={.rules[0].resources}{.rules[0].verbs}' 2>/dev/null || true)"
  if [ "$res" = '["secrets"]["create"]' ]; then
    echo "F15 verified: Role grants secret creation"
    exit 0
  fi
  sleep 10
done

echo "F15 NOT verified: no secret-create Role in $NAMESPACE/$ROLENAME" >&2
exit 1
