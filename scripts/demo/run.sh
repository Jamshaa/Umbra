#!/usr/bin/env bash
# run.sh: the guided 3-minute demo. Prints each command with a '+'
# prefix, pauses between steps (DEMO_PAUSE seconds, 0 with --fast),
# and stops at the first error. Only sanitized output is shown.
#
# Flags: --fast | --no-model | --replay
# Note on traps: the full live trap watches take ~5 minutes each
# (30x10s clean windows), so by default this script shows the static
# trap mechanisms plus the pre-measured live rows from the bench table.
# Set DEMO_TRAPS_LIVE=1 to run the full live trap watches instead.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."

FAST=0
NO_MODEL=0
REPLAY=0
for a in "$@"; do
	case "$a" in
	--fast) FAST=1 ;;
	--no-model) NO_MODEL=1 ;;
	--replay) REPLAY=1 ;;
	*) echo "run: unknown flag $a" >&2; exit 2 ;;
	esac
done
PAUSE="${DEMO_PAUSE:-3}"
[ "$FAST" = 1 ] && PAUSE=0
HOST=".build/host"
[ -x "$HOST/umbra.exe" ] && EXE=".exe" || EXE=""

step() {
	echo
	echo "== $*"
	if [ "$PAUSE" != 0 ] && [ -t 0 ]; then
		sleep "$PAUSE"
	fi
}
run() {
	echo "+ $*"
	"$@"
}

if [ "$REPLAY" = 1 ]; then
	echo "REPLAY — resultados guardados, no una ejecución en vivo"
	echo "======================================================"
	python3 -c "
import json
sb=json.load(open('bench/results/scoreboard.json'))
for r in sb.get('rows',[]):
    print(r.get('config'), r.get('group'), 'live' if r.get('live') else 'static', str(r.get('pass'))+'/'+str(r.get('cases')))
"
	if [ -f .demo/receipts.chain ]; then
		run "$HOST/umbra$EXE" receipts verify .demo/receipts.chain || true
	fi
	exit 0
fi

step "0:00 Contexto: cluster y operador"
run kubectl --context kind-umbra get nodes
run kubectl --context kind-umbra get crd

step "0:20 Inyección y detección"
run bash scripts/injectors/f01_no_limits.sh
run "$HOST/umbra$EXE" scan --namespace demo --publish

step "0:45 Quién lo cambió"
run kubectl --context kind-umbra --as=alice@example.com -n demo scale deploy/web --replicas=2
run bash scripts/demo/audit-export.sh
run "$HOST/umbra$EXE" changes --kind Deployment --name web --namespace demo --audit-file .demo/audit.log
run kubectl --context kind-umbra -n demo scale deploy/web --replicas=1

step "1:05 Puertas: parche correcto frente a incorrecto (grupo B)"
run "$HOST/umbra-bench$EXE" --mode handpatch --hp-case F36 --hp-patch bench/handpatch/F36/correct.yaml --hp-rule ServiceEndpoints --out .demo/bench --scoreboard-out .demo/scoreboard.json
run "$HOST/umbra-bench$EXE" --mode handpatch --hp-case F36 --hp-patch bench/handpatch/F36/incorrect.yaml --hp-rule ServiceEndpoints --out .demo/bench --scoreboard-out .demo/scoreboard.json

if [ "$NO_MODEL" = 1 ] || [ -z "${MODEL_API_KEY:-}" ]; then
	echo
	echo "== 1:40 Modelo: OMITIDO (sin clave o --no-model) — fila «sin medir» en la tabla"
else
	step "1:40 Modelo: diagnose F36 + model-raw"
	run bash scripts/injectors/f36_service_no_endpoints.sh
	run "$HOST/umbra-agent$EXE" diagnose --kind Service --name f36-no-endpoints --namespace demo --fault F36 --sandbox --publish --receipt-chain .demo/receipts.chain
	run "$HOST/umbra-bench$EXE" --mode model-raw --cases F36 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json
fi

step "2:15 Trampas"
if [ "${DEMO_TRAPS_LIVE:-0}" = 1 ]; then
	run "$HOST/umbra-bench$EXE" --mode deterministic --cases T1,T2,T3,T4 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json
else
	echo "Modo corto: mecanismos estáticos; los watches live completos están pre-medidos en la tabla generada."
	run "$HOST/umbra-bench$EXE" --mode deterministic --cases T1,T2,T3,T4 --out .demo/bench --scoreboard-out .demo/scoreboard.json
fi

step "2:35 Recibo y manipulación"
if [ -f .demo/receipts.chain ]; then
	run "$HOST/umbra$EXE" receipts verify .demo/receipts.chain
	run bash scripts/demo/tamper.sh
else
	echo "Sin cadena de demostración (segmento del modelo omitido): no se muestra recibo."
fi

step "2:50 UI y cierre"
echo "Abre http://127.0.0.1:8082 (umbra-api) — distintivo LIVE, hallazgos y scoreboard."
echo "Cierre: el operador guarda estado y puertas; el agente propone; lo «sin medir» se dice en voz alta."
