#!/usr/bin/env bash
# f35_impossible_affinity.sh (verifier): afirma que el Deployment demo
# F35 exige una etiqueta de nodo inexistente (Pending permanente).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f35-impossible-affinity"

for _ in $(seq 1 30); do
  key="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key}' 2>/dev/null || true)"
  if [ "$key" = "umbra.io/gpu" ]; then
    echo "F35 verified: impossible node affinity ($key)"
    exit 0
  fi
  sleep 10
done

echo "F35 NOT verified: no impossible affinity in $NAMESPACE/$DEPLOY" >&2
exit 1
