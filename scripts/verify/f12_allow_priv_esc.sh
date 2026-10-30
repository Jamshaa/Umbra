#!/usr/bin/env bash
# f12_allow_priv_esc.sh (verifier): afirma que el pod demo F12 declara
# allowPrivilegeEscalation=true con SYS_ADMIN anadida.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f12-allow-priv-esc"

for _ in $(seq 1 30); do
  spec="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].securityContext}' 2>/dev/null || true)"
  # Pod existe y el securityContext combina escalado + SYS_ADMIN
  # (jsonpath lo serializa como JSON, con la clave entrecomillada).
  if printf '%s' "$spec" | grep -q '"allowPrivilegeEscalation":true' \
    && printf '%s' "$spec" | grep -q 'SYS_ADMIN'; then
    echo "F12 verified: allowPrivilegeEscalation with SYS_ADMIN"
    exit 0
  fi
  sleep 10
done

echo "F12 NOT verified: no escalating pod in $NAMESPACE/$LABEL" >&2
exit 1
