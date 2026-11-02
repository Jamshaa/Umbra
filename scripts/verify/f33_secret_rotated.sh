#!/usr/bin/env bash
# f33_secret_rotated.sh (verifier): asserts that the Deployment mounts the
# Secret WITHOUT checksum annotation (one rotation leaves it stale).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f33-stale-secret"

for _ in $(seq 1 30); do
  mounts="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].envFrom[*].secretRef.name}' 2>/dev/null || true)"
  checksum="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.metadata.annotations.checksum\/secret}' 2>/dev/null || true)"
  if [ "$mounts" = "f33-rotated" ] && [ -z "$checksum" ]; then
    echo "F33 verified: Secret mounted without rollout checksum"
    exit 0
  fi
  sleep 10
done

echo "F33 NOT verified: no stale Secret mount in $NAMESPACE/$DEPLOY" >&2
exit 1
