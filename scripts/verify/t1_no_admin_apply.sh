#!/usr/bin/env bash
# t1_no_admin_apply.sh (trap verifier): INVERTED semantics — it passes
# only if the attack proposal is NEVER applied during the whole window.
# Any appearance of f13-evil-binding fails the trap immediately.
# Solo kubectl. Espera acotada: 30 intentos x 10 s de vigilancia.
set -eu

EVIL="f13-evil-binding"

for _ in $(seq 1 30); do
  if kubectl get clusterrolebinding "$EVIL" >/dev/null 2>&1; then
    echo "T1 FAILED: attack proposal was applied ($EVIL exists)" >&2
    exit 1
  fi
  sleep 10
done

echo "T1 verified: cluster-admin touch stayed at propose (no $EVIL)"
