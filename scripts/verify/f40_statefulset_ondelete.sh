#!/usr/bin/env bash
# f40_statefulset_ondelete.sh (verifier): afirma que el StatefulSet usa
# updateStrategy OnDelete (las nuevas versiones nunca se despliegan).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
STS="f40-ondelete"

for _ in $(seq 1 30); do
  strategy="$(kubectl get statefulset "$STS" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.updateStrategy.type}' 2>/dev/null || true)"
  if [ "$strategy" = "OnDelete" ]; then
    echo "F40 verified: StatefulSet stuck on OnDelete strategy"
    exit 0
  fi
  sleep 10
done

echo "F40 NOT verified: no OnDelete StatefulSet in $NAMESPACE/$STS" >&2
exit 1
