#!/usr/bin/env bash
# f38_minutely_cronjob.sh (verifier): afirma que el CronJob demo F38
# corre cada minuto ("* * * * *"). Solo kubectl, solo ns demo.
# Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
CRONJOB="f38-minutely"

for _ in $(seq 1 30); do
  sched="$(kubectl --context kind-umbra get cronjob "$CRONJOB" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.schedule}' 2>/dev/null || true)"
  if [ "$sched" = "* * * * *" ]; then
    echo "F38 verified: CronJob hammers every minute ($sched)"
    exit 0
  fi
  sleep 10
done

echo "F38 NOT verified: no minutely CronJob in $NAMESPACE/$CRONJOB" >&2
exit 1
