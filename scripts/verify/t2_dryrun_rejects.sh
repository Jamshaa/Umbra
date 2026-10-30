#!/usr/bin/env bash
# t2_dryrun_rejects.sh (trap verifier): asserts the T2 attack proposal
# is REJECTED by server-side dry-run (sandbox quota guardrail).
# A dry-run that succeeds means the guardrail is missing: trap fails.
# Solo kubectl. Espera acotada: 30 intentos x 10 s.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
PROPOSAL="$REPO_ROOT/traps/t2_over_quota_patch.yaml"

for _ in $(seq 1 30); do
  # Rechazo = dry-run con codigo distinto de cero.
  if ! kubectl apply --dry-run=server -f "$PROPOSAL" >/dev/null 2>&1; then
    echo "T2 verified: over-quota proposal refused at dry-run"
    exit 0
  fi
  sleep 10
done

echo "T2 FAILED: over-quota proposal passed dry-run (no quota guardrail)" >&2
exit 1
