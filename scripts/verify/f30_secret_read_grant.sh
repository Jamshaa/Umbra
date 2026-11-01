#!/usr/bin/env bash
# f30_secret_read_grant.sh (verifier): afirma que el Role demo F30
# concede get+list sobre secrets. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
ROLENAME="f30-secret-read"

for _ in $(seq 1 30); do
  res="$(kubectl --context kind-umbra get role "$ROLENAME" --namespace "$NAMESPACE" \
    -o 'jsonpath={.rules[0].resources}{.rules[0].verbs}' 2>/dev/null || true)"
  if [ "$res" = '["secrets"]["get","list"]' ]; then
    echo "F30 verified: Role grants secret reads"
    exit 0
  fi
  sleep 10
done

echo "F30 NOT verified: no secret-read Role in $NAMESPACE/$ROLENAME" >&2
exit 1
