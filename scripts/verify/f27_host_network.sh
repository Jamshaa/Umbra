#!/usr/bin/env bash
# f27_host_network.sh (verifier): afirma que el pod demo F27 corre con
# hostNetwork activado. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
DEPLOY="f27-host-network"

for _ in $(seq 1 30); do
  host="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.hostNetwork}' 2>/dev/null || true)"
  if [ "$host" = "true" ]; then
    echo "F27 verified: pod shares the host network namespace"
    exit 0
  fi
  sleep 10
done

echo "F27 NOT verified: no hostNetwork pod in $NAMESPACE/$DEPLOY" >&2
exit 1
