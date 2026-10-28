#!/usr/bin/env bash
# up.sh: prepara el namespace demo para los injectores F01-F05.
# Crea el namespace demo (si no existe) y limpia restos de
# inyecciones anteriores dentro de ESE namespace. No toca nada mas.
# Idempotente: re-ejecutar es seguro.
# Unico binario externo: kubectl (en PATH).
set -eu

NAMESPACE="demo"

# El namespace se declara en el propio repo (demo/kustomization.yaml
# fija `namespace: demo`), pero los injectores no presuponen que exista.
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create namespace "$NAMESPACE"
fi

# Limpia restos de inyecciones anteriores: solo los Deployments
# de fallos conocidos dentro del namespace demo. Nada mas.
kubectl delete deployment f01-no-limits f02-low-memory f03-huge-requests f04-no-limits-bursty f05-no-liveness \
  --namespace "$NAMESPACE" --ignore-not-found=true
