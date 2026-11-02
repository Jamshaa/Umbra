#!/usr/bin/env bash
# f21_missing_pull_secret.sh (verifier): asserts that demo pod F21 uses
# a private-registry image without imagePullSecrets.
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f21-missing-pull-secret"

for _ in $(seq 1 30); do
  img="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].image}' 2>/dev/null || true)"
  secrets="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.imagePullSecrets[*].name}' 2>/dev/null || true)"
  # Private image (ghcr.io) present with no pull secrets.
  if printf '%s' "$img" | grep -q 'ghcr\.io/' && [ -z "$secrets" ]; then
    echo "F21 verified: private image without pull secrets ($img)"
    exit 0
  fi
  sleep 10
done

echo "F21 NOT verified: no unauthenticated private image in $NAMESPACE/$LABEL" >&2
exit 1
