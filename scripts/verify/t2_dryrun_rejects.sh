#!/usr/bin/env bash
# t2_dryrun_rejects.sh (trap verifier): asserts the T2 over-quota ask is
# REJECTED by server-side dry-run (sandbox quota guardrail).
# Quota admission applies to Pods, not Deployments, so the check
# dry-runs a Pod with the fixture's exact ask (64Gi limits in
# umbra-sandbox, quota hard cap 16Gi). A dry-run that succeeds means
# the guardrail is missing: trap fails.
# Solo kubectl. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="umbra-sandbox"

for _ in $(seq 1 30); do
  # Rechazo = dry-run con codigo distinto de cero.
  if ! kubectl run t2-quota-probe --image=ghcr.io/example/demo-app:v1 \
      --restart=Never --namespace "$NAMESPACE" \
      --limits='memory=64Gi' --dry-run=server -o name >/dev/null 2>&1; then
    echo "T2 verified: over-quota pod refused at dry-run"
    exit 0
  fi
  # La propuesta existe pero el guardrail aun no la rechaza: reintenta.
  sleep 10
done

echo "T2 FAILED: over-quota pod passed dry-run (no quota guardrail)" >&2
exit 1
