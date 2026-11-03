#!/usr/bin/env bash
# preflight.sh: demo readiness, in order. Stops at the first failure
# and says what to do. Never prints secret values.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
fail() { echo "preflight: $*" >&2; exit 1; }

docker ps >/dev/null 2>&1 || fail "docker is not responding (start Docker Desktop)"
kind_bin="${KIND_BIN:-$HOME/go/bin/kind}"
[ -x "$kind_bin" ] || kind_bin="$(command -v kind || true)"
[ -n "$kind_bin" ] || fail "kind not found (expected at ~/go/bin/kind)"
"$kind_bin" get clusters 2>/dev/null | grep -qx umbra || fail "kind cluster 'umbra' does not exist (bash scripts/kind/create-cluster.sh)"

CTX_NOW="$(kubectl config current-context 2>/dev/null || echo none)"
[ "$CTX_NOW" = "kind-umbra" ] || fail "current context is '$CTX_NOW', want kind-umbra"

kubectl --context kind-umbra get crd findings.umbra.io >/dev/null 2>&1 ||
	fail "umbra.io CRDs not installed (kubectl apply -k config/crd)"
kubectl --context kind-umbra -n umbra-system get deploy umbra-manager >/dev/null 2>&1 ||
	fail "manager not deployed (bash scripts/demo/up.sh)"
kubectl --context kind-umbra -n umbra-system rollout status deploy/umbra-manager --timeout=30s >/dev/null 2>&1 ||
	fail "manager is not Ready"

# RBAC the manager actually needs (a resourceNames rule denies general
# list/watch, which is how this once CrashLooped in silence). Fail here
# with a clear message instead of on camera.
SA="system:serviceaccount:umbra-system:umbra-manager"
for check in "list findings.umbra.io" "list remediationpolicies.umbra.io"; do
	# shellcheck disable=SC2086
	[ "$(kubectl --context kind-umbra auth can-i $check --as="$SA" 2>/dev/null)" = "yes" ] ||
		fail "manager cannot $check (kubectl apply -k config/rbac)"
done
for res in networkpolicies resourcequotas; do
	[ "$(kubectl --context kind-umbra -n umbra-sandbox auth can-i list "$res" --as="$SA" 2>/dev/null)" = "yes" ] ||
		fail "manager cannot list $res in umbra-sandbox (kubectl apply -k config/rbac)"
done
[ "$(kubectl --context kind-umbra -n umbra-sandbox auth can-i update networkpolicies/deny-all --as="$SA" 2>/dev/null)" = "yes" ] ||
	fail "manager cannot update the sandbox deny-all policy (kubectl apply -k config/rbac)"

NP="$(kubectl --context kind-umbra -n umbra-sandbox get networkpolicy default-deny -o name 2>/dev/null || true)"
[ -n "$NP" ] || fail "sandbox deny-all NetworkPolicy missing (bash scripts/demo/up.sh)"

# Sandbox network probe: a pod inside umbra-sandbox must NOT reach out.
# BLOCKED is the only acceptable outcome. Create + wait + logs (no
# attach: attached runs add TTY noise that hides the verdict).
kubectl --context kind-umbra -n umbra-sandbox delete pod umbra-preflight-probe --ignore-not-found >/dev/null 2>&1 || true
kubectl --context kind-umbra -n umbra-sandbox run umbra-preflight-probe --image=busybox:1.36 --restart=Never --command -- wget -q -T 5 -O /dev/null http://example.com >/dev/null 2>&1 || true
kubectl --context kind-umbra -n umbra-sandbox wait --for=jsonpath='{.status.phase}'=Failed pod/umbra-preflight-probe --timeout=90s >/dev/null 2>&1 || true
PROBE="$(kubectl --context kind-umbra -n umbra-sandbox logs umbra-preflight-probe 2>&1 || true)"
kubectl --context kind-umbra -n umbra-sandbox delete pod umbra-preflight-probe --ignore-not-found >/dev/null 2>&1 || true
case "$PROBE" in
*BLOCKED* | *"can't connect"* | *"bad address"* | *timed\ out* | *unreachable* | *"Network is unreachable"*) echo "sandbox egress: BLOCKED" ;;
*) echo "preflight: sandbox egress probe outcome: $PROBE" >&2; fail "sandbox egress was not clearly BLOCKED" ;;
esac

for b in umbra umbra-agent umbra-bench umbra-api; do
	[ -x ".build/host/$b" ] || [ -x ".build/host/$b.exe" ] || fail "missing .build/host/$b (bash scripts/build-bins.sh)"
done
[ -f ui/dist/index.html ] || fail "ui/dist not built (npm run build:live --prefix ui)"
if grep -r -q -E 'googleapis|gstatic' ui/dist; then
	fail "ui/dist references external font hosts (self-hosted fonts expected)"
fi

[ -n "${UMBRA_RECEIPT_HMAC_KEY:-}" ] || fail "UMBRA_RECEIPT_HMAC_KEY is not set (see SECURITY.md, ephemeral per session)"
if [ -z "${MODEL_API_KEY:-}" ]; then
	echo "preflight: MODEL_API_KEY not set; the model segment will be skipped"
fi
echo "preflight: OK"
