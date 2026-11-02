# META.md — how Umbra uses Meta models

## Model and API

- **Model:** Muse Spark (`muse-spark-1.3`, configurable via `MODEL_NAME`).
- **API:** Responses API (`POST {MODEL_BASE_URL}/responses`), recommended by
  Meta for agents because it preserves reasoning across tool
  calls via `previous_response_id`.
- **Client:** `internal/brain/brain.go` (commit `d6fabd4`, wave T1). `tool_choice`
  is always `"auto"`: the allowlist is enforced by code
  (`internal/agent/tools.go`), never the model. `reasoning_effort` is never sent:
  Muse Spark always reasons and `reasoning_effort: none` returns 400.
- **Configurable base URL:** `MODEL_BASE_URL` (default
  `https://api.meta.ai/v1`) accepts an OpenAI-compatible server for the
  local variant (Muse Glimmer); see ROADMAP.md v0.3.

## Where it's used

| File | Role |
|---|---|
| `internal/brain/brain.go` | Transport: 1 round-trip, retries only on 429/5xx/network failures, hash cache, `Usage` (tokens, latency, response id) per call |
| `internal/agent/loop.go` | Loop: anti-injection system prompt, `MaxSteps` 8, token budget, `propose_patch` as sole output |
| `internal/agent/tools.go` | 8 tools: `get_manifest`, `get_events`, `get_pod_status`, `get_logs` (sanitized), `get_related`, `get_changes`, `get_impact` + `propose_patch` |
| `cmd/umbra-agent/main.go` | Live wiring: real kube → loop → gates (`--sandbox` or memory-only) |
| `cmd/umbra-bench/main.go` | `model` mode (gates on) and `model-raw` (gates off, measures only) |

## When the model is actually needed

Only ambiguous causes: CrashLoop (F07), broken references (F16–F18),
mismatched ports (F20). F01–F06 have a deterministic fix and **never
go through the model** (`internal/fixes`).

## Tokens per run

Measured by the bench itself (`tokens_total` in `bench/results/scoreboard.json`).
No live measurement published yet: the README table will only publish figures
from `bench/results/*.jsonl`. Anti-burn rule: `MaxTokensPerFinding`,
identical-turn cache, and if credits run tight drop to 2
repeats or fewer cases (see BENCHMARK.md).
