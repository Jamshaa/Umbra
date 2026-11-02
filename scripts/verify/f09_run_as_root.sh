#!/usr/bin/env bash
# f09_run_as_root.sh (verifier): asserts that demo pod F09 runs without
# runAsNonRoot (i.e. root by default). kubectl only,
# demo ns. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f09-run-as-root"

for _ in $(seq 1 30); do
  pods="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].metadata.name}' 2>/dev/null || true)"
  nonroot="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].securityContext.runAsNonRoot}' 2>/dev/null || true)"
  # Pod exists and no container declares runAsNonRoot=true.
  if [ -n "$pods" ] && ! printf '%s' "$nonroot" | grep -q 'true'; then
    echo "F09 verified: container runs without runAsNonRoot (root by default)"
    exit 0
  fi
  sleep 10
done

echo "F09 NOT verified: no root-by-default pod in $NAMESPACE/$LABEL" >&2
exit 1
