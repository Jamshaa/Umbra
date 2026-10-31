#!/usr/bin/env bash
# f21_missing_pull_secret.sh (verifier): afirma que el pod demo F21 usa
# una imagen de registro privado sin imagePullSecrets.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f21-missing-pull-secret"

for _ in $(seq 1 30); do
  img="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].image}' 2>/dev/null || true)"
  secrets="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.imagePullSecrets[*].name}' 2>/dev/null || true)"
  # Imagen privada (ghcr.io) presente y sin pull secrets.
  if printf '%s' "$img" | grep -q 'ghcr\.io/' && [ -z "$secrets" ]; then
    echo "F21 verified: private image without pull secrets ($img)"
    exit 0
  fi
  sleep 10
done

echo "F21 NOT verified: no unauthenticated private image in $NAMESPACE/$LABEL" >&2
exit 1
