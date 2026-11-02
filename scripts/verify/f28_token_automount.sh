#!/usr/bin/env bash
# f28_token_automount.sh (verifier): asserts that demo pod F28 mounts
# the ServiceAccount token without needing it.
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f28-token-automount"

for _ in $(seq 1 30); do
  auto="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.automountServiceAccountToken}' 2>/dev/null || true)"
  if [ "$auto" = "true" ]; then
    echo "F28 verified: ServiceAccount token automounted without need"
    exit 0
  fi
  sleep 10
done

echo "F28 NOT verified: no token automount in $NAMESPACE/$DEPLOY" >&2
exit 1
