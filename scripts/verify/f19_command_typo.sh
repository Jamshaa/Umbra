#!/usr/bin/env bash
# f19_command_typo.sh (verifier): asserts that demo pod F19 never starts
# because the executable does not exist (runtime "not found" message).
# kubectl only, demo ns only. Bounded wait: 30 tries x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
LABEL="app=f19-command-typo"

for _ in $(seq 1 30); do
  # The runtime message names the missing executable; the reason
  # varies across runtimes (CrashLoopBackOff/CreateContainerError).
  msg="$(kubectl --context kind-umbra get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].status.containerStatuses[*].state.waiting.message}{.items[*].status.containerStatuses[*].lastState.terminated.message}' 2>/dev/null || true)"
  if printf '%s' "$msg" | grep -qi 'executable.*not found\|not found.*executable\|exec.*no such file'; then
    echo "F19 verified: container fails on typo'd executable"
    exit 0
  fi
  sleep 10
done

echo "F19 NOT verified: no executable-not-found failure in $NAMESPACE/$LABEL" >&2
exit 1
