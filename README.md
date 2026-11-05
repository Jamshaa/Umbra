# Umbra

**Indie · Build for Your World · Apache-2.0 ([LICENSE](LICENSE))**
Built during the event: [docs/NEW_WORK.md](docs/NEW_WORK.md) · AI use: [docs/AI_USE.md](docs/AI_USE.md) · Model: [docs/META.md](docs/META.md)

> Self-healing Kubernetes configs, end to end:
> detect → diagnose → patch → verify in sandbox → roll back on failure.

**Status: pre-release.** Nothing reaches production without a deterministic test.
Function-by-function status: [docs/STATUS.md](docs/STATUS.md). Demo: [docs/DEMO.md](docs/DEMO.md).

## What it does

- Scanner reports a misconfiguration with attribution (who changed what).
- Agent (Muse Spark via Responses API) proposes a typed patch, never applies it.
- Sandbox dry-runs the patch: pass → human approves, fail → snapshot rollback.

Unlike K8sGPT / HolmesGPT (explain only) or Aurora (PRs only), Umbra tests
the patch before applying and leaves a receipt. The model proposes,
code verifies, a human approves.

## How it works

```
Scanner → Agent (read-only + propose_patch) → Sandbox (dry-run + apply)
  │              │ allowlist + invariants              │ pass → propose (human approves)
  │              │                                     │ fail → rollback via snapshot
  └── finding with attribution (audit log / managedFields)
```

Safety: code-enforced tool list (`internal/agent/tools.go`), typed patches
against an allowlist, secrets redacted before the prompt (`internal/evidence`),
no auto-apply, minimal read-only RBAC. Details: `SECURITY.md`, `docs/META.md`.

## Quickstart

```bash
bash scripts/kind/create-cluster.sh   # 1. kind cluster + audit log
bash scripts/build-images.sh          # 2. kind-only images
kubectl apply -k config/crd           # 3. CRDs
make bench                            # 4. deterministic scoreboard row
cd ui && npm ci && npm run build      # 5. UI
```

Or `make demo` for everything at once. `MODEL_API_KEY` is only needed for
model modes; evidence sent to the model is sanitized.

## Benchmark

Seed 42, 3 repeats (`bench/cases.yaml`). Groups: **A** = deterministic fix,
**B** = model-only fix, **C** = diagnosis only, **T** = traps.
`live` marks kind-verified rows. Method: `docs/BENCHMARK.md`.
Table below is generated from `bench/results/scoreboard.json` — do not edit by hand.

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

Raw data: `bench/results/*.jsonl`, `bench/results/scoreboard.json`.

## Docs

Limits: [docs/LIMITATIONS.md](docs/LIMITATIONS.md) · Roadmap: [ROADMAP.md](ROADMAP.md) ·
Contributing: [CONTRIBUTING.md](CONTRIBUTING.md) · Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
