#!/usr/bin/env bash
# f12_allow_priv_esc.sh (verifier): asserts that demo pod F12 declares
# allowPrivilegeEscalation=true with SYS_ADMIN added.
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f12-allow-priv-esc"

for _ in $(seq 1 30); do
  spec="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].securityContext}' 2>/dev/null || true)"
  # Pod exists and the securityContext combines escalation + SYS_ADMIN
  # (jsonpath serializes it as JSON, with the key quoted).
  if printf '%s' "$spec" | grep -q '"allowPrivilegeEscalation":true' \
    && printf '%s' "$spec" | grep -q 'SYS_ADMIN'; then
    echo "F12 verified: allowPrivilegeEscalation with SYS_ADMIN"
    exit 0
  fi
  sleep 10
done

echo "F12 NOT verified: no escalating pod in $NAMESPACE/$LABEL" >&2
exit 1
