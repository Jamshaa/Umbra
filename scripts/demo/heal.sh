#!/usr/bin/env bash
# heal.sh: return shop-api to its healthy manifest so a take can be
# repeated. Idempotent. Demo namespace only.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
kubectl --context kind-umbra apply -f demo/live/shop-api.yaml >/dev/null
kubectl --context kind-umbra -n demo rollout status deploy/shop-api --timeout=60s >/dev/null
echo "heal: shop-api healthy (64Mi, probes present)"
