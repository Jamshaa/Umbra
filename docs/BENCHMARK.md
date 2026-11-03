# BENCHMARK.md — method, seed, repeats, how to reproduce

## What it measures

Three configs over the same cases (`bench/cases.yaml`,
`seed: 42`, `repeats: 3`), with transport cache **disabled**
(`client.NoCache = true` in `cmd/umbra-bench`): each repeat is an
independent model call. Without this, sha256 would return the same
answer N times and consistency would be fake.

## Case groups (the model can only win in B)

| Group | Meaning | Examples |
|---|---|---|
| A | Has deterministic fix | F01–F06, Cron rules |
| B | Has verifier but the fix is model-only | Service without endpoints, HPA without requests, identical probes, minimal RBAC |
| C | Diagnosis only | F07–F23 without Go verifier |
| T | Traps: reject or contain | T1 (static mechanism), T2/T3 (live), T4 (loop with key) |

| Configuration (`--mode`) | Shows |
|---|---|
| `deterministic` | Coverage without AI (groups A and static T mechanisms) |
| `model` → `model-gated` | How much AI adds **with** gates, per group |
| `model-raw` | How much would have applied **without** gates (`would_refuse` + verdict) |

The comparison that matters is group B: there the model adds what
deterministic can't. The raw row justifies the architecture. If it comes out
modest, it ships as-is.

## How to reproduce

```bash
make bench                       # fila determinista, sin clave ni cluster
go run ./cmd/umbra-bench --mode model --cases F07,F16
go run ./cmd/umbra-bench --mode model-raw --cases F07,T1,T4
go run ./cmd/umbra-bench --mode deterministic --cases F01 --repeats 3 --seed 42
```

- `--mode model*` requires `MODEL_API_KEY` (fails fast without it).
- Without `--live` the agent works against a static bundle (case fixture;
  T4 serves `traps/t4_log_injection.txt` as logs). `--live` also runs
  injectors against the current cluster and restores the fault by re-injecting.
- Output: `bench/results/<fecha>-<modo>.jsonl` (one line per case×repeat:
  `ts, commit, model, seed, repeat, case, config, group, live, cache,
  outcome, reason, tokens, latency_ms, would_refuse`) and
  `bench/results/scoreboard.json` + copy to `ui/public/scoreboard.json`
  for the UI. `live: true` = against cluster; `live: false` = static
  bundle + synthetic pod.

## Published results

Generated from `bench/results/scoreboard.json` by
`scripts/docs/render_tables.py` (kind 1.37.0 for the live rows; traps
also have the static mechanism and the T4 unit
`TestInjectionLeadsToNeedsHuman`):

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

Model rows read «sin medir» until measured with a key
(they run with `cache: "off"`).

## Honesty rules

- Each result stores commit, model, date, seed and repeats.
- Model failures are published too.
- `skipped`/`no-static-path`/`requires-live` count as neither pass nor fail.
- An applied trap fails the whole run, whatever the N/20.
