#!/usr/bin/env bash
# dashboard.sh: install the Kubernetes Dashboard (pinned v2.7.0, single
# manifest, no Helm) plus a demo admin user, and print how to reach it.
# Re-runnable: kubectl apply is idempotent, the admin user is created
# only if missing. kind-umbra context only (guard.sh, no override).
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

DASHBOARD_VERSION="v2.7.0"
DASHBOARD_URL="https://raw.githubusercontent.com/kubernetes/dashboard/${DASHBOARD_VERSION}/aio/deploy/recommended.yaml"

kubectl --context kind-umbra apply -f "$DASHBOARD_URL"
kubectl --context kind-umbra -n kubernetes-dashboard rollout status deploy/kubernetes-dashboard --timeout=180s

if ! kubectl --context kind-umbra -n kubernetes-dashboard get serviceaccount admin-user >/dev/null 2>&1; then
	kubectl --context kind-umbra -n kubernetes-dashboard create serviceaccount admin-user
fi
if ! kubectl --context kind-umbra get clusterrolebinding dashboard-admin >/dev/null 2>&1; then
	kubectl --context kind-umbra create clusterrolebinding dashboard-admin \
		--clusterrole=cluster-admin --serviceaccount=kubernetes-dashboard:admin-user
fi

# The demo flow this enables (Umbra side, all repo tooling):
#   1. break a Deployment YAML in the dashboard (e.g. demo/f01-no-limits:
#      delete the resources block) and save it there,
#   2. ./.build/host/umbra scan --namespace demo --publish
#      -> the Finding appears (dashboard: Custom Resources > findings),
#   3. serve the live UI (npm run build:live --prefix ui plus umbra-api)
#      to see the finding, its sandbox diff and the timeline.
echo
echo "Dashboard installed (pinned ${DASHBOARD_VERSION}). Reach it with:"
echo "  kubectl --context kind-umbra proxy"
echo "  http://127.0.0.1:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/"
echo "Login with the token printed by:"
echo "  kubectl --context kind-umbra -n kubernetes-dashboard create token admin-user --duration=24h"
