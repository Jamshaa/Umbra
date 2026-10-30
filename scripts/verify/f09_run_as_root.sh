#!/usr/bin/env bash
# f09_run_as_root.sh (verifier): afirma que el pod demo F09 corre sin
# runAsNonRoot (es decir, como root por defecto). Solo kubectl, solo
# ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
LABEL="app=f09-run-as-root"

for _ in $(seq 1 30); do
  pods="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].metadata.name}' 2>/dev/null || true)"
  nonroot="$(kubectl get pods --namespace "$NAMESPACE" -l "$LABEL" \
    -o 'jsonpath={.items[*].spec.containers[*].securityContext.runAsNonRoot}' 2>/dev/null || true)"
  # Pod existe y ningún contenedor declara runAsNonRoot=true.
  if [ -n "$pods" ] && ! printf '%s' "$nonroot" | grep -q 'true'; then
    echo "F09 verified: container runs without runAsNonRoot (root by default)"
    exit 0
  fi
  sleep 10
done

echo "F09 NOT verified: no root-by-default pod in $NAMESPACE/$LABEL" >&2
exit 1
