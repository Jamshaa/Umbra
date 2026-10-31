#!/usr/bin/env bash
# f29_plaintext_secret.sh (verifier): afirma que el Deployment demo F29
# fija un valor literal en una variable con nombre de secreto.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
DEPLOY="f29-plaintext-secret"

for _ in $(seq 1 30); do
  val="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].env[?(@.name=="DB_PASSWORD")].value}' 2>/dev/null || true)"
  if [ -n "$val" ]; then
    echo "F29 verified: plaintext secret value in manifest"
    exit 0
  fi
  sleep 10
done

echo "F29 NOT verified: no literal secret value in $NAMESPACE/$DEPLOY" >&2
exit 1
