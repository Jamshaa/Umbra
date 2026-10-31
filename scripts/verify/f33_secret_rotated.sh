#!/usr/bin/env bash
# f33_secret_rotated.sh (verifier): afirma que el Deployment monta el
# Secret SIN anotacion checksum (una rotacion lo deja obsoleto).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
DEPLOY="f33-stale-secret"

for _ in $(seq 1 30); do
  mounts="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].envFrom[*].secretRef.name}' 2>/dev/null || true)"
  checksum="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.metadata.annotations.checksum\/secret}' 2>/dev/null || true)"
  if [ "$mounts" = "f33-rotated" ] && [ -z "$checksum" ]; then
    echo "F33 verified: Secret mounted without rollout checksum"
    exit 0
  fi
  sleep 10
done

echo "F33 NOT verified: no stale Secret mount in $NAMESPACE/$DEPLOY" >&2
exit 1
