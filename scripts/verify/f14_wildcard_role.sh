#!/usr/bin/env bash
# f14_wildcard_role.sh (verifier): afirma que el Role demo F14 concede
# "*" en recursos y verbos. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
ROLENAME="f14-wildcard"

for _ in $(seq 1 30); do
  # Ambas listas son exactamente ["*"].
  res="$(kubectl get role "$ROLENAME" --namespace "$NAMESPACE" \
    -o 'jsonpath={.rules[0].resources}{.rules[0].verbs}' 2>/dev/null || true)"
  if [ "$res" = "[*][*]" ]; then
    echo "F14 verified: Role grants wildcard resources and verbs"
    exit 0
  fi
  sleep 10
done

echo "F14 NOT verified: no wildcard Role in $NAMESPACE/$ROLENAME" >&2
exit 1
