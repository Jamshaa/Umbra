#!/usr/bin/env bash
# f40_statefulset_ondelete.sh (verifier): asserts that the StatefulSet uses
# updateStrategy OnDelete (new versions never deploy).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
STS="f40-ondelete"

for _ in $(seq 1 30); do
  strategy="$(kubectl --context kind-umbra get statefulset "$STS" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.updateStrategy.type}' 2>/dev/null || true)"
  if [ "$strategy" = "OnDelete" ]; then
    echo "F40 verified: StatefulSet stuck on OnDelete strategy"
    exit 0
  fi
  sleep 10
done

echo "F40 NOT verified: no OnDelete StatefulSet in $NAMESPACE/$STS" >&2
exit 1
