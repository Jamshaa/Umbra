#!/usr/bin/env bash
# f23_statefulset_no_headless.sh (verifier): asserts that the Service
# f23-missing-headless does NOT exist (the StatefulSet has no headless).
# Absence is the steady state: 3 probes suffice.
# kubectl only, demo ns only.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"

for _ in $(seq 1 3); do
  if ! kubectl --context kind-umbra get service f23-missing-headless --namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo "F23 verified: StatefulSet without headless Service"
    exit 0
  fi
  sleep 10
done

echo "F23 NOT verified: headless Service exists in $NAMESPACE" >&2
exit 1
