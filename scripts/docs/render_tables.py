#!/usr/bin/env python3
"""Render bench result tables into docs from bench/results/scoreboard.json.

Stdlib only. The scoreboard is the single source of truth for published
numbers: docs must never carry hand-written result figures outside the
markers this script owns.

Usage:
    python3 scripts/docs/render_tables.py          # write tables into docs
    python3 scripts/docs/render_tables.py --check  # exit 1 if any doc differs
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCOREBOARD = ROOT / "bench" / "results" / "scoreboard.json"
DOCS = [
    ROOT / "README.md",
    ROOT / "docs" / "STATUS.md",
    ROOT / "docs" / "BENCHMARK.md",
    ROOT / "docs" / "DEMO.md",
]
START = "<!-- umbra:bench-table:start -->"
END = "<!-- umbra:bench-table:end -->"
CONFIGS = ["deterministic", "model-gated", "model-raw", "handpatch"]
GROUPS = ["A", "B", "C", "T"]


def render_table(sb):
    repeats = int(sb.get("repeats") or 1)
    top_commit = str(sb.get("commit") or "")
    by_key = {}
    for row in sb.get("rows") or []:
        by_key[(row.get("config"), row.get("group"), bool(row.get("live")))] = row
    lines = [
        "| Configuración | Grupo | Live | Resultado pass/total (N × R) | Rechazados | Omitidos | Tokens | Commit |",
        "|---|---|---|---|---|---|---|---|",
    ]
    for cfg in CONFIGS:
        for group in GROUPS:
            for live in (True, False):
                row = by_key.get((cfg, group, live))
                live_s = "true" if live else "false"
                if row is None:
                    lines.append(
                        f"| {cfg} | {group} | {live_s} | sin medir | sin medir | sin medir | sin medir | sin medir |"
                    )
                    continue
                cases = int(row.get("cases") or 0)
                passed = int(row.get("pass") or 0)
                # cases counts pass+fail records. Express as distinct
                # cases x repeats only when the row is a homogeneous
                # repeats run; otherwise report records as-is (N x 1).
                if repeats > 1 and cases and cases % repeats == 0:
                    n, r = cases // repeats, repeats
                else:
                    n, r = cases, 1
                commit = str(row.get("commit") or top_commit or "")
                lines.append(
                    f"| {cfg} | {group} | {live_s} | {passed}/{cases} ({n} × {r}) "
                    f"| {int(row.get('refused') or 0)} | {int(row.get('skipped') or 0)} "
                    f"| {int(row.get('tokens_total') or 0)} | {commit} |"
                )
    return "\n".join(lines)


def replace_table(text, table):
    start = text.find(START)
    end = text.find(END)
    if start < 0 or end < 0 or end < start:
        return None
    end += len(END)
    return text[:start] + START + "\n" + table + "\n" + END + text[end:]


def main(argv):
    check = "--check" in argv
    sb = json.loads(SCOREBOARD.read_text(encoding="utf-8"))
    table = render_table(sb)
    bad = []
    for doc in DOCS:
        text = doc.read_text(encoding="utf-8")
        new = replace_table(text, table)
        if new is None:
            bad.append(f"{doc.relative_to(ROOT)}: missing bench-table markers")
            continue
        if new != text:
            if check:
                bad.append(f"{doc.relative_to(ROOT)}: bench table out of date")
            else:
                doc.write_text(new, encoding="utf-8")
                print(f"updated {doc.relative_to(ROOT)}")
    if bad:
        for b in bad:
            print("render_tables:", b, file=sys.stderr)
        return 1
    print("bench tables up to date" if check else "bench tables written")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
