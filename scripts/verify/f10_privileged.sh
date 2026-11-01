#!/usr/bin/env bash
# f10_privileged.sh (verifier): afirma que el pod demo F10 declara
# securityContext.privileged=true. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f10-privileged"

for _ in $(seq 1 30); do
  priv="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].securityContext.privileged}' 2>/dev/null || true)"
  # Pod existe y algún contenedor declara privileged=true.
  if printf '%s' "$priv" | grep -q 'true'; then
    echo "F10 verified: container runs privileged"
    exit 0
  fi
  sleep 10
done

echo "F10 NOT verified: no privileged pod in $NAMESPACE/$LABEL" >&2
exit 1
