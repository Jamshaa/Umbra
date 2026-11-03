# STATUS.md — the only table that matters

Three columns per feature. **Live** = run against kind or against the
model with a key. Unit test with fixtures doesn't count as live. The README
links here and doesn't repeat claims.

## Core

| Feature | Implemented | Unit tests | Live verified |
|---|---|---|---|
| F01–F06, F21 detectors | Yes | Yes | F01–F06: faults confirmed in kind (F02 OOMKill loop, F03 Pending); rest no |
| Cron detectors (deadline, suspend, overlap, tz, bounds, history, GPU) | Yes | Yes | No (no cluster) |
| Graph detectors (Service, HPA, RBAC, Namespace) | Yes | Yes | F36 live (Service without endpoints); inverted F26 inadmissible in etcd → static only |
| F01–F06 + Cron + graph verifiers | Yes | Yes | F01–F06 via sandbox (see generated bench table below) |
| F01–F06, Cron fixes | Yes | Yes (Fix→Verify chained) | F01–F06 applied in sandbox and reverted |
| Gate ladder + closed fail | Yes | Yes | Yes: dry-run + apply + rollback in `umbra-sandbox` per case |
| Isolated sandbox (deny-all) | Yes (`config/sandbox/`) | — | Yes: BLOCKED inside, REACHED outside |
| Sanitizer + T4 | Yes | Yes (incl. injection) | T4 loop needs key |
| Chained receipt + `receipts verify` | Yes | Yes | No |
| Self-declared approval + `umbra-approver` | Yes | Yes | No (no live audit) |
| Attribution incl. impersonation | Yes | Yes | Yes: kind logs `user` + `impersonatedUser`; `Present()` labels it |
| Responses API client + loop + 8 tools | Yes | Yes (fake kube + scripted model) | No (no key) |
| `umbra-agent diagnose`, `umbra-bench` 3 modes | Yes | No (binaries; bench validated statically) | Deterministic yes; model no |
| Scoreboard UI + labeled live/mock timeline | Yes | Build+lint | No live data |
| Bench rows by config/group/live | Yes (JSONL + scoreboard) | — | See the generated bench table below (kind 1.37.0) |
| Handpatch group B (4 correct + 4 incorrect) | Yes (`bench/handpatch/`) | — | `live: true`: correct pass, incorrect rejected with gate and reason |

## Add-on (code exists, live evidence pending)

| Feature | Implemented | Unit tests | Live verified |
|---|---|---|---|
| Real PR to GitHub | Plan-only | Yes | No (no GH_TOKEN) |
| Template mining | Yes | Yes | No |
| Change-incident correlation | Yes | Yes | No |
| Impact graph | Yes | Yes | No |
| CronJob calendar | Partial (`NextCronRuns` + overlap) | Yes | Analyzed live (Jobs every minute on the dot; sleep-90 with Allow overlaps 2 active); no bench harness |
| `model-raw` (evaluation mode; operator shadow mode no existe) | Yes | — | No (no key) |

## Unbuilt design (ROADMAP.md)

Helm/chart, measured local model, SIEM, Slack/Teams, dual approval,
MCP server, vCluster, multi-cluster, Prometheus metrics, kubectl plugin (roadmap, not built),
full supply-chain (Scorecard), envtest, fuzzing, e2e in CI, operator shadow
mode, restart recovery, `install.yaml` install (roadmap, not built).

## Bench results (generated, do not edit numbers)

Source: `bench/results/scoreboard.json`, rendered by
`scripts/docs/render_tables.py`.

<!-- umbra:bench-table:start -->
| Configuración | Grupo | Live | Resultado pass/total (N × R) | Rechazados | Omitidos | Tokens | Commit |
|---|---|---|---|---|---|---|---|
| deterministic | A | true | 21/21 (7 × 3) | 0 | 0 | 0 | a0ad509 |
| deterministic | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| deterministic | B | true | 1/1 (1 × 1) | 0 | 3 | 0 | a0ad509 |
| deterministic | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| deterministic | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| deterministic | C | false | 3/3 (1 × 3) | 0 | 42 | 0 | a0ad509 |
| deterministic | T | true | 12/12 (4 × 3) | 0 | 0 | 0 | a0ad509 |
| deterministic | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | A | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | B | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | C | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | T | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | A | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | B | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | C | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | T | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | A | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | B | true | 0/4 (4 × 1) | 0 | 0 | 0 | a0ad509 |
| handpatch | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | C | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | T | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
<!-- umbra:bench-table:end -->

## Naming notes
- `model-raw` = bench **evaluation** mode (what would have applied without
  gates). Not the operator shadow mode (no existe: report-only shadow
  mode doesn't exist).
- "Operator": the reconciler holds state and gates; the agent
  (`umbra-agent`) proposes and the bench measures. No apply autonomy.
