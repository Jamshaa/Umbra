#!/usr/bin/env bash
# f08_readiness_mismatch.sh (verifier): asserts that demo pod F08 is
# Running but with readiness false (mismatch 8081 vs 8080).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f08-readiness-mismatch"

for _ in $(seq 1 30); do
  if kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
      -o 'jsonpath={range .items[*]}{.status.phase} ready={range .status.containerStatuses[*]}{.ready}{end}{"\n"}{end}' 2>/dev/null \
      | grep -q '^Running ready=false$'; then
    echo "F08 verified: Running but never Ready (readiness mismatch)"
    exit 0
  fi
  sleep 10
done

echo "F08 NOT verified: no Running-but-unready pod in $NAMESPACE/$LABEL" >&2
exit 1
