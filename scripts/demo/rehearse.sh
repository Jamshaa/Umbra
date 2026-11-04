#!/usr/bin/env bash
# rehearse.sh: full demo rehearsal A..G with wall-clock timing and
# expected-result checks. Logs to .demo/rehearsal-<date>.log.
#   rehearse.sh [--fast]   (--fast: no pauses; Act E = T2 live + saved)
# Acts mirror bench/demo-set.yaml exactly. Any act failure is recorded
# and the rehearsal still reports every act; exit code is 1 if any failed.
# No cluster is touched except through the guarded demo scripts/binaries.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
mkdir -p .demo
LOG=".demo/rehearsal-$(date +%Y%m%d-%H%M%S).log"
FAST=0
for a in "$@"; do
	case "$a" in
	--fast) FAST=1 ;;
	*) echo "rehearse: unknown flag $a" >&2; exit 2 ;;
	esac
done
export DEMO_PAUSE=$([ "$FAST" = 1 ] && echo 0 || echo "${DEMO_PAUSE:-3}")
exec > >(tee -a "$LOG") 2>&1
echo "rehearsal: $(date -Is) fast=$FAST log=$LOG"

FAILED=""
run_act() { # $1=letter $2=fn
	local start dur rc
	start=$(date +%s)
	echo "== ACT $1 start $(date -Is)"
	set +e
	"$2"
	rc=$?
	set -e
	dur=$(($(date +%s) - start))
	if [ "$rc" = 0 ]; then
		echo "ACT $1: OK ${dur}s (fuente: reloj de rehearse.sh)"
	else
		echo "ACT $1: FAIL ${dur}s rc=$rc (fuente: reloj de rehearse.sh)"
		FAILED="$FAILED $1"
	fi
	[ "$FAST" = 1 ] || sleep "$DEMO_PAUSE"
}

act_a() {
	[ -n "${MODEL_API_KEY:-}" ] || { echo "BLOQUEADO: sin modelo no hay demo completa"; return 1; }
	[ -n "${UMBRA_RECEIPT_HMAC_KEY:-}" ] || { echo "BLOQUEADO: falta UMBRA_RECEIPT_HMAC_KEY"; return 1; }
	bash scripts/demo/break.sh f36
	./.build/host/umbra scan --namespace demo --publish
	# Real form: umbra-agent parses flags from argv[1] (no `diagnose` word).
	./.build/host/umbra-agent --kind Service --name f36-no-endpoints --namespace demo --fault F36 \
		--sandbox --publish --receipt-chain .demo/receipts.chain
}
act_b() {
	bash scripts/demo/break.sh f02
	./.build/host/umbra-bench --mode deterministic --cases F02 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json
	bash scripts/demo/heal.sh
}
act_c() {
	kubectl --context kind-umbra --as=alice@example.com -n demo scale deploy/shop-api --replicas=2
	bash scripts/demo/audit-export.sh
	./.build/host/umbra changes --kind Deployment --name shop-api --namespace demo --audit-file .demo/audit.log
	kubectl --context kind-umbra -n demo scale deploy/shop-api --replicas=1
}
act_d() {
	./.build/host/umbra-bench --mode handpatch --hp-case F36 --hp-patch bench/handpatch/F36/incorrect.yaml --hp-rule ServiceEndpoints --out .demo/bench --scoreboard-out .demo/scoreboard.json
	./.build/host/umbra-bench --mode handpatch --hp-case F36 --hp-patch bench/handpatch/F36/correct.yaml --hp-rule ServiceEndpoints --out .demo/bench --scoreboard-out .demo/scoreboard.json
	if [ -n "${MODEL_API_KEY:-}" ]; then
		./.build/host/umbra-bench --mode model-raw --cases F36 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json
	else
		echo "model-raw: sin medir (sin clave)"
	fi
}
act_e() {
	./.build/host/umbra-bench --mode deterministic --cases T2 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json
	bash scripts/demo/show-saved.sh
}
act_f() {
	[ -f .demo/receipts.chain ] || { echo "sin cadena del acto A: no se muestra recibo"; return 1; }
	./.build/host/umbra receipts verify .demo/receipts.chain
	bash scripts/demo/tamper.sh
}
act_g() {
	npm run build:live --prefix ui
	echo "UI: abrir la servida por umbra-api (distintivo LIVE) y comprobar gate_verdict en el detalle"
}

bash scripts/demo/preflight.sh
bash scripts/demo/reset.sh
bash scripts/demo/heal.sh
run_act A act_a
run_act B act_b
run_act C act_c
run_act D act_d
run_act E act_e
run_act F act_f
run_act G act_g

if [ -n "$FAILED" ]; then
	echo "rehearsal: FAILED acts:$FAILED (log $LOG)"
	exit 1
fi
echo "rehearsal: all acts OK (log $LOG)"
