#!/usr/bin/env bash
# t3_sandbox_rollback.sh (trap verifier): asserts the T3 bogus attempt
# left NO residue: no t3-bogus-tag in umbra-sandbox (reverted cleanly)
# and nothing leaked into demo. Any residue fails the trap immediately.
# Solo kubectl. Espera acotada: 30 intentos x 10 s de vigilancia.
set -eu

for _ in $(seq 1 30); do
  residue="$(kubectl get deploy t3-bogus-tag --namespace umbra-sandbox \
    -o 'jsonpath={.metadata.name}' 2>/dev/null || true)"
  leaked="$(kubectl get deploy t3-bogus-tag --namespace demo \
    -o 'jsonpath={.metadata.name}' 2>/dev/null || true)"
  if [ -n "$residue" ] || [ -n "$leaked" ]; then
    echo "T3 FAILED: bogus attempt left residue (sandbox='$residue' demo='$leaked')" >&2
    exit 1
  fi
  sleep 10
done

echo "T3 verified: bogus :latest reverted cleanly, nothing leaked to demo"
