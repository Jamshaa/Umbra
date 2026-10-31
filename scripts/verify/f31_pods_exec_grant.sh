#!/usr/bin/env bash
# f31_pods_exec_grant.sh (verifier): afirma que el Role demo F31 concede
# create sobre pods/exec. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
ROLENAME="f31-pods-exec"

for _ in $(seq 1 30); do
  res="$(kubectl get role "$ROLENAME" --namespace "$NAMESPACE" \
    -o 'jsonpath={.rules[0].resources}{.rules[0].verbs}' 2>/dev/null || true)"
  if [ "$res" = '["pods/exec"]["create"]' ]; then
    echo "F31 verified: Role grants pods/exec"
    exit 0
  fi
  sleep 10
done

echo "F31 NOT verified: no pods/exec Role in $NAMESPACE/$ROLENAME" >&2
exit 1
