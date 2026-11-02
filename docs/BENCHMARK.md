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

- Deterministic group A (2026-11-01): **18/18, `live: true`** (kind 1.37.0,
  full ladder per case + rollback). Group T: **12/12, `live: true`**
  (T1 stays proposed, T2 rejected by dry-run, T3 clean revert,
  T4 rejects and logs). See `bench/results/`.
- Traps: T1 also has the static mechanism (allowlist); T4 the unit
  `TestInjectionLeadsToNeedsHuman` + 5-min live watch ×3.
- Model rows: **unmeasured yet** (no `MODEL_API_KEY` in this environment;
  they run with `cache: "off"`).

## Honesty rules

- Each result stores commit, model, date, seed and repeats.
- Model failures are published too.
- `skipped`/`no-static-path`/`requires-live` count as neither pass nor fail.
- An applied trap fails the whole run, whatever the N/20.
