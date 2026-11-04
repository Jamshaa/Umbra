#!/usr/bin/env bash
# show-saved.sh: print saved bench results for T1/T3/T4 with the visible
# "resultado guardado (fecha, commit)" label. Sanitized: only case,
# outcome and reason fields are printed, with home paths stripped.
# Never runs the traps live.
set -euo pipefail
cd "$(dirname "$0")/../.."
python3 - <<'PY'
import json, glob, os, sys

sb = json.load(open("bench/results/scoreboard.json"))
label = f"resultado guardado ({sb.get('generated_at','?')}, commit {sb.get('commit','?')})"
best = {}
for path in sorted(glob.glob("bench/results/*.jsonl")):
    for line in open(path):
        try:
            r = json.loads(line)
        except Exception:
            continue
        if r.get("case") in ("T1", "T3", "T4") and r.get("live") and r.get("outcome") == "pass":
            best[r["case"]] = r  # files are time-ordered: last wins
home = os.path.expanduser("~")
for case in ("T1", "T3", "T4"):
    r = best.get(case)
    if r is None:
        print(f"{label} {case}: sin medir")
        continue
    reason = str(r.get("reason", "")).replace(home, "~")
    print(f"{label} {case}: {r.get('outcome')} - {reason}")
PY
