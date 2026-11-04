#!/usr/bin/env bash
# break.sh: reproduce, on shop-api (or the F36 fixture), what the
# presenter would do by hand. Idempotent. Demo namespace only.
#   break.sh f36|f02|f01
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
case "${1:-}" in
f36)
	bash scripts/injectors/f36_service_no_endpoints.sh
	;;
f02)
	kubectl --context kind-umbra apply -f demo/live/shop-api.yaml >/dev/null
	kubectl --context kind-umbra -n demo patch deploy/shop-api \
		-p '{"spec":{"template":{"spec":{"containers":[{"name":"app","resources":{"limits":{"memory":"8Mi"}}}]}}}}' >/dev/null
	echo "break: shop-api memory limit set to 8Mi (F02)"
	;;
f01)
	kubectl --context kind-umbra apply -f demo/live/shop-api.yaml >/dev/null
	kubectl --context kind-umbra -n demo patch deploy/shop-api --type json \
		-p='[{"op":"remove","path":"/spec/template/spec/containers/0/resources"}]' >/dev/null
	echo "break: shop-api resources removed (F01 fallback)"
	;;
*)
	echo "usage: break.sh f36|f02|f01" >&2
	exit 2
	;;
esac
