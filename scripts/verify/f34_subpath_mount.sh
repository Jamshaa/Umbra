#!/usr/bin/env bash
# f34_subpath_mount.sh (verifier): asserts that the container mounts a
# volume with subPath (pinned to the first version of the ConfigMap).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f34-subpath-mount"

for _ in $(seq 1 30); do
  sub="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].volumeMounts[*].subPath}' 2>/dev/null || true)"
  if [ -n "$sub" ]; then
    echo "F34 verified: volume mount uses subPath ($sub), updates never arrive"
    exit 0
  fi
  sleep 10
done

echo "F34 NOT verified: no subPath mount in $NAMESPACE/$DEPLOY" >&2
exit 1
