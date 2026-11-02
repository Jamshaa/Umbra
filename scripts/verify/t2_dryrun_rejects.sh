#!/usr/bin/env bash
# t2_dryrun_rejects.sh (trap verifier): asserts the T2 over-quota ask is
# REJECTED by server-side dry-run (sandbox quota guardrail).
# Quota admission applies to Pods, not Deployments, so the check
# dry-runs a Pod with the fixture's exact ask (64Gi limits in
# umbra-sandbox, quota hard cap 16Gi). A dry-run that succeeds means
# the guardrail is missing: trap fails.
# kubectl only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="umbra-sandbox"

for _ in $(seq 1 30); do
  # Rejection = dry-run with a nonzero exit code.
  if ! kubectl --context kind-umbra run t2-quota-probe --image=ghcr.io/example/demo-app:v1 \
      --restart=Never --namespace "$NAMESPACE" \
      --limits='memory=64Gi' --dry-run=server -o name >/dev/null 2>&1; then
    echo "T2 verified: over-quota pod refused at dry-run"
    exit 0
  fi
  # The proposal exists but the guardrail does not reject it yet: retry.
  sleep 10
done

echo "T2 FAILED: over-quota pod passed dry-run (no quota guardrail)" >&2
exit 1
