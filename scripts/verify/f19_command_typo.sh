#!/usr/bin/env bash
# f19_command_typo.sh (verifier): afirma que el pod demo F19 no arranca
# porque el ejecutable no existe (mensaje "not found" del runtime).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f19-command-typo"

for _ in $(seq 1 30); do
  # El mensaje del runtime menciona el ejecutable ausente; el motivo
  # varia entre runtimes (CrashLoopBackOff/CreateContainerError).
  msg="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.message}{.items[*].status.containerStatuses[*].lastState.terminated.message}' 2>/dev/null || true)"
  if printf '%s' "$msg" | grep -qi 'executable.*not found\|not found.*executable\|exec.*no such file'; then
    echo "F19 verified: container fails on typo'd executable"
    exit 0
  fi
  sleep 10
done

echo "F19 NOT verified: no executable-not-found failure in $NAMESPACE/$LABEL" >&2
exit 1
