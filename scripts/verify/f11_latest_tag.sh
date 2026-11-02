#!/usr/bin/env bash
# f11_latest_tag.sh (verifier): asserts that demo pod F11 uses a
# image with :latest tag (or untagged). kubectl only, demo ns only.
# Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f11-latest-tag"

for _ in $(seq 1 30); do
  img="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].image}' 2>/dev/null || true)"
  # Image present ending in :latest (or untagged, same thing).
  if [ -n "$img" ] && { printf '%s' "$img" | grep -q ':latest$'; }; then
    echo "F11 verified: image uses mutable :latest tag ($img)"
    exit 0
  fi
  sleep 10
done

echo "F11 NOT verified: no :latest image pod in $NAMESPACE/$LABEL" >&2
exit 1
