#!/usr/bin/env bash
# f32_configmap_stale.sh (verifier): afirma que el Deployment monta el
# ConfigMap SIN anotacion checksum (cualquier cambio lo deja obsoleto).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
DEPLOY="f32-stale-config"

for _ in $(seq 1 30); do
  mounts="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].envFrom[*].configMapRef.name}' 2>/dev/null || true)"
  checksum="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.metadata.annotations.checksum\/config}' 2>/dev/null || true)"
  if [ "$mounts" = "f32-settings" ] && [ -z "$checksum" ]; then
    echo "F32 verified: ConfigMap mounted without rollout checksum"
    exit 0
  fi
  sleep 10
done

echo "F32 NOT verified: no stale ConfigMap mount in $NAMESPACE/$DEPLOY" >&2
exit 1
