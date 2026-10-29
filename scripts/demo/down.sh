#!/usr/bin/env bash
# down.sh: retira el namespace demo por completo (con todo su contenido).
# No toca ningun otro namespace. Idempotente: re-ejecutar es seguro.
# Unico binario externo: kubectl (en PATH).
set -eu

NAMESPACE="demo"

kubectl delete namespace "$NAMESPACE" --ignore-not-found=true
