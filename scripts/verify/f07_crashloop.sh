#!/usr/bin/env bash
# f07_crashloop.sh (verifier): afirma que el pod demo F07 esta en
# CrashLoopBackOff con al menos un reinicio. Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s. Sale 0 si verifica, 1 si no.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f07-crashloop"

for _ in $(seq 1 30); do
  if kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
      -o 'jsonpath={range .items[*].status.containerStatuses[*]}{.state.waiting.reason} restarts={.restartCount}{"\n"}{end}' 2>/dev/null \
      | grep -q 'CrashLoopBackOff restarts=[1-9]'; then
    echo "F07 verified: CrashLoopBackOff with restarts"
    exit 0
  fi
  sleep 10
done

echo "F07 NOT verified: no CrashLoopBackOff pod with restarts in $NAMESPACE/$LABEL" >&2
exit 1
