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

# Sandbox network variant (N0/N1): this probe is an AVISO, never a
# failure. N1 only if every check shows the expected isolation; anything
# else is N0 with the literal detail in .demo/network.txt. In N0 the demo
# must not claim "aislado" or "red denegada": it says "separate sandbox
# namespace with quota, copy and rollback".
mkdir -p .demo
wait_phase() { # $1=ns $2=pod -> prints phase ("" on timeout)
	local i ph
	for i in $(seq 1 30); do
		ph="$(kubectl --context kind-umbra -n "$1" get pod "$2" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
		case "$ph" in Succeeded | Failed) echo "$ph"; return 0 ;; esac
		sleep 2
	done
	echo ""
}
classify() { # $1=phase $2=logs -> BLOCKED|REACHED|INCONCLUSIVE
	case "$1" in
	Succeeded) echo REACHED ;;
	Failed)
		case "$2" in
		*BLOCKED* | *"can't connect"* | *"bad address"* | *timed\ out* | *unreachable* | *"Network is unreachable"*) echo BLOCKED ;;
		*) echo INCONCLUSIVE ;;
		esac
		;;
	*) echo INCONCLUSIVE ;;
	esac
}
# Target with no NetworkPolicy (default ns) + control pod on the same side.
kubectl --context kind-umbra -n default delete pod umbra-preflight-target umbra-preflight-control --ignore-not-found >/dev/null 2>&1 || true
kubectl --context kind-umbra -n umbra-sandbox delete pod umbra-preflight-probe umbra-preflight-pod2pod --ignore-not-found >/dev/null 2>&1 || true
kubectl --context kind-umbra -n default run umbra-preflight-target --image=busybox:1.36 --restart=Never --command -- httpd -f -p 8080 >/dev/null 2>&1 || true
TARGET_IP=""
for i in $(seq 1 30); do
	TARGET_IP="$(kubectl --context kind-umbra -n default get pod umbra-preflight-target -o jsonpath='{.status.podIP}' 2>/dev/null || true)"
	[ -n "$TARGET_IP" ] && break
	sleep 2
done
probe_pod() { # $1=ns $2=pod $3=url -> "<phase> <logs>"
	kubectl --context kind-umbra -n "$1" delete pod "$2" --ignore-not-found >/dev/null 2>&1 || true
	kubectl --context kind-umbra -n "$1" run "$2" --image=busybox:1.36 --restart=Never --command -- wget -q -T 5 -O /dev/null "$3" >/dev/null 2>&1 || true
	local ph
	ph="$(wait_phase "$1" "$2")"
	local logs
	logs="$(kubectl --context kind-umbra -n "$1" logs "$2" 2>&1 || true)"
	echo "$ph $logs"
}
EGRESS_RAW="$(probe_pod umbra-sandbox umbra-preflight-probe http://example.com)"
EGRESS="$(classify "${EGRESS_RAW%% *}" "$EGRESS_RAW")"
CONTROL="INCONCLUSIVE"
POD2POD="INCONCLUSIVE"
if [ -n "$TARGET_IP" ]; then
	CONTROL_RAW="$(probe_pod default umbra-preflight-control "http://${TARGET_IP}:8080")"
	CONTROL="$(classify "${CONTROL_RAW%% *}" "$CONTROL_RAW")"
	POD2POD_RAW="$(probe_pod umbra-sandbox umbra-preflight-pod2pod "http://${TARGET_IP}:8080")"
	POD2POD="$(classify "${POD2POD_RAW%% *}" "$POD2POD_RAW")"
	[ "$CONTROL" != "REACHED" ] && POD2POD="INCONCLUSIVE"
fi
kubectl --context kind-umbra -n default delete pod umbra-preflight-target umbra-preflight-control --ignore-not-found >/dev/null 2>&1 || true
kubectl --context kind-umbra -n umbra-sandbox delete pod umbra-preflight-probe umbra-preflight-pod2pod --ignore-not-found >/dev/null 2>&1 || true
NET="N0"
if [ "$EGRESS" = "BLOCKED" ] && [ "$POD2POD" = "BLOCKED" ] && [ "$CONTROL" = "REACHED" ]; then
	NET="N1"
fi
{
	echo "$NET"
	echo "egress sandbox->example.com: $EGRESS (raw: $EGRESS_RAW)"
	echo "pod->pod sandbox->target-sin-politica: $POD2POD"
	echo "control sin politica->target: $CONTROL"
} >.demo/network.txt
if [ "$NET" = "N1" ]; then
	echo "sandbox network: N1 (aislamiento verificado)"
else
	echo "preflight: AVISO red N0: aislamiento no verificado (ver .demo/network.txt); la demo dice: separate sandbox namespace with quota, copy and rollback" >&2
fi

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
