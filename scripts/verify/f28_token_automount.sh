#!/usr/bin/env bash
# f28_token_automount.sh (verifier): afirma que el pod demo F28 monta
# el token de la ServiceAccount sin necesitarlo.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f28-token-automount"

for _ in $(seq 1 30); do
  auto="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.automountServiceAccountToken}' 2>/dev/null || true)"
  if [ "$auto" = "true" ]; then
    echo "F28 verified: ServiceAccount token automounted without need"
    exit 0
  fi
  sleep 10
done

echo "F28 NOT verified: no token automount in $NAMESPACE/$DEPLOY" >&2
exit 1
