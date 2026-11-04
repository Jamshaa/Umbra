#!/usr/bin/env bash
# heal.sh: restore a healthy stage so a take can be repeated.
#   heal.sh          -> shop-api healthy (default, as before)
#   heal.sh f02|f01  -> shop-api healthy
#   heal.sh f36      -> delete the two F36 resources (Service
#                       f36-no-endpoints, Deployment f36-app, ns demo;
#                       names from demo/f36_service_no_endpoints.yaml).
#                       Nothing is reapplied: break.sh f36 re-creates
#                       both from scratch (injector runs kubectl apply),
#                       so break -> heal -> break chains cleanly.
# Idempotent. Demo namespace only.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
case "${1:-shop-api}" in
shop-api | f02 | f01)
	kubectl --context kind-umbra apply -f demo/live/shop-api.yaml >/dev/null
	kubectl --context kind-umbra -n demo rollout status deploy/shop-api --timeout=60s >/dev/null
	echo "heal: shop-api healthy (64Mi, probes present)"
	;;
f36)
	kubectl --context kind-umbra -n demo delete service f36-no-endpoints --ignore-not-found >/dev/null
	kubectl --context kind-umbra -n demo delete deployment f36-app --ignore-not-found >/dev/null
	echo "heal: F36 resources removed (break.sh f36 re-creates them)"
	;;
*)
	echo "usage: heal.sh [f36|f02|f01]" >&2
	exit 2
	;;
esac
