#!/usr/bin/env bash
# f11_latest_tag.sh (verifier): afirma que el pod demo F11 usa una
# imagen con tag :latest (o sin tag). Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f11-latest-tag"

for _ in $(seq 1 30); do
  img="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].image}' 2>/dev/null || true)"
  # Imagen presente y termina en :latest (o sin tag, que equivale).
  if [ -n "$img" ] && { printf '%s' "$img" | grep -q ':latest$'; }; then
    echo "F11 verified: image uses mutable :latest tag ($img)"
    exit 0
  fi
  sleep 10
done

echo "F11 NOT verified: no :latest image pod in $NAMESPACE/$LABEL" >&2
exit 1
