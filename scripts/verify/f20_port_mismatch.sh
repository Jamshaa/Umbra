#!/usr/bin/env bash
# f20_port_mismatch.sh (verifier): afirma que el Deployment demo F20
# declara probes en un puerto (9090) que el contenedor no abre (8080).
# Es una propiedad del spec: no depende de timings del pod.
# Solo kubectl, solo ns demo. Espera acotada: 30 intentos x 10 s.
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"
DEPLOY="f20-port-mismatch"

for _ in $(seq 1 30); do
  spec="$(kubectl --context kind-umbra get deploy "$DEPLOY" --namespace "$NAMESPACE" \
    -o 'jsonpath={.spec.template.spec.containers[0].ports[*].containerPort}/{.spec.template.spec.containers[0].livenessProbe.tcpSocket.port}/{.spec.template.spec.containers[0].readinessProbe.tcpSocket.port}' 2>/dev/null || true)"
  # Puertos abiertos 8080 frente a probes en 9090: mismatch.
  if [ "$spec" = "8080/9090/9090" ]; then
    echo "F20 verified: probes target 9090 while container opens 8080"
    exit 0
  fi
  sleep 10
done

echo "F20 NOT verified: no port mismatch in $NAMESPACE/$DEPLOY" >&2
exit 1
