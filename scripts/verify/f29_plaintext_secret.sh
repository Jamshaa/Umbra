#!/usr/bin/env bash
# f29_plaintext_secret.sh (verifier): asserts that demo Deployment F29
# pins a literal value in a secret-named variable.
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f29-plaintext-secret"

for _ in $(seq 1 30); do
  val="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].env[?(@.name=="DB_PASSWORD")].value}' 2>/dev/null || true)"
  if [ -n "$val" ]; then
    echo "F29 verified: plaintext secret value in manifest"
    exit 0
  fi
  sleep 10
done

echo "F29 NOT verified: no literal secret value in $NAMESPACE/$DEPLOY" >&2
exit 1
