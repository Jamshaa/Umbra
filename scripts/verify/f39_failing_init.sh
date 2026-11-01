#!/usr/bin/env bash
# f39_failing_init.sh (verifier): afirma que el Deployment demo F39
# declara un initContainer (que falla siempre: pod en Initializing).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f39-failing-init"

for _ in $(seq 1 30); do
  init="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.initContainers[*].name}' 2>/dev/null || true)"
  if [ -n "$init" ]; then
    echo "F39 verified: failing initContainer present ($init)"
    exit 0
  fi
  sleep 10
done

echo "F39 NOT verified: no initContainer in $NAMESPACE/$DEPLOY" >&2
exit 1
