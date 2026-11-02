#!/usr/bin/env bash
# f24_cronjob_suspended.sh (verifier): asserts that demo CronJob F24
# is suspended (schedule silently stopped).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
CRONJOB="f24-suspended"

for _ in $(seq 1 30); do
  susp="$(kubectl --context kind-umbra get cronjob "$CRONJOB" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.suspend}' 2>/dev/null || true)"
  if [ "$susp" = "true" ]; then
    echo "F24 verified: CronJob suspended, schedule silently stopped"
    exit 0
  fi
  sleep 10
done

echo "F24 NOT verified: CronJob not suspended in $NAMESPACE/$CRONJOB" >&2
exit 1
