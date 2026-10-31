#!/usr/bin/env bash
# f34_subpath_mount.sh (verifier): afirma que el contenedor monta un
# volumen con subPath (anclado a la primera version del ConfigMap).
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu

NAMESPACE="demo"
DEPLOY="f34-subpath-mount"

for _ in $(seq 1 30); do
  sub="$(kubectl get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[*].volumeMounts[*].subPath}' 2>/dev/null || true)"
  if [ -n "$sub" ]; then
    echo "F34 verified: volume mount uses subPath ($sub), updates never arrive"
    exit 0
  fi
  sleep 10
done

echo "F34 NOT verified: no subPath mount in $NAMESPACE/$DEPLOY" >&2
exit 1
