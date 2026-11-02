#!/usr/bin/env bash
# f32_configmap_stale.sh (verifier): asserts that the Deployment mounts the
# ConfigMap WITHOUT checksum annotation (any change leaves it stale).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f32-stale-config"

for _ in $(seq 1 30); do
  mounts="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].envFrom[*].configMapRef.name}' 2>/dev/null || true)"
  checksum="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.metadata.annotations.checksum\/config}' 2>/dev/null || true)"
  if [ "$mounts" = "f32-settings" ] && [ -z "$checksum" ]; then
    echo "F32 verified: ConfigMap mounted without rollout checksum"
    exit 0
  fi
  sleep 10
done

echo "F32 NOT verified: no stale ConfigMap mount in $NAMESPACE/$DEPLOY" >&2
exit 1
