#!/usr/bin/env bash
# f15_secret_create_grant.sh (verifier): afirma que el Role demo F15
# concede create sobre secrets. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
ROLENAME="f15-secret-create"

for _ in $(seq 1 30); do
  # recursos == [secrets] y verbos contienen create.
  res="$(kubectl get role "$ROLENAME" --namespace "$NAMESPACE" \
    -o 'jsonpath={.rules[0].resources}{.rules[0].verbs}' 2>/dev/null || true)"
  if [ "$res" = "[secrets][create]" ]; then
    echo "F15 verified: Role grants secret creation"
    exit 0
  fi
  sleep 10
done

echo "F15 NOT verified: no secret-create Role in $NAMESPACE/$ROLENAME" >&2
exit 1
