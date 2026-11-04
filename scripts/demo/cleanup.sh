#!/usr/bin/env bash
# cleanup.sh: HUMAN-RUN cluster cleanup for the demo. Lists everything it
# would delete, requires the typed word BORRAR, then deletes and shows
# the final state. Idempotent. kind-umbra only (guard.sh + --context).
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
CTX="--context kind-umbra"
TYPES="deploy,svc,cm,secret,sa,role,rolebinding,cronjob,job,pod,pdb,hpa"

echo "== Se BORRARIA =="
echo "-- namespace demo (todo salvo deploy/shop-api):"
# shellcheck disable=SC2086
kubectl $CTX -n demo get $TYPES --ignore-not-found 2>/dev/null | grep -v '^deployment.apps/shop-api' || true
echo "-- todos los Finding:"
kubectl $CTX get findings --ignore-not-found 2>/dev/null || true
echo "-- namespaces de pruebas (np-*):"
kubectl $CTX get ns -o name 2>/dev/null | grep -E '^namespace/np-' || echo "(ninguno)"
echo "-- contenido de umbra-sandbox (se conserva el namespace):"
# shellcheck disable=SC2086
kubectl $CTX -n umbra-sandbox get $TYPES --ignore-not-found 2>/dev/null || true

printf 'Escribe BORRAR para confirmar: '
read -r answer
[ "$answer" = "BORRAR" ] || { echo "cleanup: cancelado"; exit 1; }

# demo: delete every deployment except shop-api, then the rest by type.
for name in $(kubectl $CTX -n demo get deploy -o name 2>/dev/null | sed 's|deployment.apps/||'); do
	[ "$name" = "shop-api" ] || kubectl $CTX -n demo delete deploy "$name" --ignore-not-found >/dev/null
done
# shellcheck disable=SC2086
kubectl $CTX -n demo delete svc,cm,secret,sa,role,rolebinding,cronjob,job,pod,pdb,hpa --all --ignore-not-found >/dev/null 2>&1 || true
kubectl $CTX delete findings --all --ignore-not-found >/dev/null 2>&1 || true
for ns in $(kubectl $CTX get ns -o name 2>/dev/null | grep -E '^namespace/np-' | sed 's|namespace/||' || true); do
	kubectl $CTX delete namespace "$ns" --ignore-not-found >/dev/null
done
# shellcheck disable=SC2086
kubectl $CTX -n umbra-sandbox delete $TYPES --all --ignore-not-found >/dev/null 2>&1 || true

echo "== Estado final =="
kubectl $CTX get all -A
kubectl $CTX get findings
