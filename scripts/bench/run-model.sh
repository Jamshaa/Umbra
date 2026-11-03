#!/usr/bin/env bash
# run-model.sh: measured model rows, run by a human with their own key.
# Never prints the key. Stops at the first integration error; bench
# results are only written by umbra-bench after a full run, so a stopped
# run leaves no partial rows behind.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
YES=0
CASES="F36"
for a in "$@"; do
	case "$a" in
	--yes) YES=1 ;;
	--cases=*) CASES="${a#--cases=}" ;;
	--cases) echo "run-model: --cases needs a value (comma-separated)" >&2; exit 2 ;;
	*) echo "run-model: unknown argument $a (use --yes and/or --cases=F36,F...)" >&2; exit 2 ;;
	esac
done
# Live-measurable group B cases with a Go verifier are listed here.
# F26 is NOT one of them: the API server refuses to persist an inverted
# HPA, so it can only ever be measured statically and must never spend
# model tokens in a live run. Identical-probes / minimal-RBAC have Go
# verifiers but no live catalog injector (handpatch-only today), so
# they cannot join this list until they get catalog cases.
[ -n "$CASES" ] || { echo "run-model: empty case list" >&2; exit 2; }
HOST=".build/host"
[ -x "$HOST/umbra.exe" ] && EXE=".exe" || EXE=""

[ -x "$HOST/umbra-agent$EXE" ] || { echo "run-model: build host binaries first (bash scripts/build-bins.sh)" >&2; exit 1; }
if [ -z "${MODEL_API_KEY:-}" ]; then
	echo "run-model: MODEL_API_KEY is not set; export it in this shell only" >&2
	exit 1
fi

echo "== ping (minimal model call)"
PING="$("$HOST/umbra-agent$EXE" ping)"
echo "$PING"
echo "$PING" | grep -q "function_call: get_manifest" || {
	echo "run-model: no function_call from the model; fix integration before spending credits" >&2
	exit 1
}

echo "== diagnose F36 (one measured run)"
bash scripts/injectors/f36_service_no_endpoints.sh
DIAG="$("$HOST/umbra-agent$EXE" diagnose --kind Service --name f36-no-endpoints --namespace demo --fault F36 --sandbox)"
echo "$DIAG"
TOKENS="$(echo "$DIAG" | grep -o '"tokens": [0-9]*' | grep -o '[0-9]*' || echo 0)"
echo "diagnose tokens: $TOKENS (full run estimate: roughly tokens x cases x repeats x modes; measure, do not guess)"

if [ "$YES" != 1 ]; then
	printf "Continue with the full measured run? [y/N] "
	read -r ans
	case "$ans" in y | Y) ;; *) echo "run-model: stopped by user"; exit 0 ;; esac
fi

# Group B cases whose verifier is Go and whose fault can exist live
# (default F36 ServiceEndpoints). Three repeats, live. The case list is
# an argument (--cases=...), never hand-edited per run.
"$HOST/umbra-bench$EXE" --mode model --cases "$CASES" --live
"$HOST/umbra-bench$EXE" --mode model-raw --cases "$CASES" --live
python3 scripts/docs/render_tables.py
echo "run-model: measured rows written; docs tables updated"
