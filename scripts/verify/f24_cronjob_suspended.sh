#!/usr/bin/env bash
# f24_cronjob_suspended.sh (verifier): afirma que el CronJob demo F24
# esta suspendido (schedule detenido en silencio).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
CRONJOB="f24-suspended"

for _ in $(seq 1 30); do
  susp="$(kubectl get cronjob "$CRONJOB" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.suspend}' 2>/dev/null || true)"
  if [ "$susp" = "true" ]; then
    echo "F24 verified: CronJob suspended, schedule silently stopped"
    exit 0
  fi
  sleep 10
done

echo "F24 NOT verified: CronJob not suspended in $NAMESPACE/$CRONJOB" >&2
exit 1
