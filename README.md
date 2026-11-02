# Umbra

> **Self-healing Kubernetes configurations, end to end:
> detect → diagnose → patch → apply in a sandbox → verify → roll back if anything fails.**

**Status: pre-release.** No fix reaches production without a deterministic test; no manual change goes unattributed.

> Function-by-function status (implemented / unit / live): [`docs/STATUS.md`](docs/STATUS.md). This README claims nothing that table doesn't back.

> UI screenshot: pending the first recorded demo (see `docs/DEMO.md`).

## The problem

Platform teams live with misconfiguration: 78% of Red Hat
respondents reported config failures in the last year. The
real flow is always the same: something breaks, the first question is
"what changed just before?", and the manual fix that follows leaves no
auditable trace. Umbra fits that day-to-day: the finding arrives with who
changed it, the proposal with its test, and the apply with receipt and rollback.

## What sets it apart

| Tool | Does | Doesn't do (Umbra does) |
|---|---|---|
| K8sGPT | Scans and explains; alpha, limited auto-remediation | Test the patch before applying |
| HolmesGPT | Read-only investigation | Propose + verify + receipt |
| Aurora | Opens PRs only after human approval | Deterministic gates and snapshot rollback |

Thesis: the model proposes, code verifies, a human approves, and everything leaves
a receipt. Honest split: **the operator holds state and
gates; the agent proposes**. Not an autonomous operator: with
no agent proposal there is nothing to verify, and with no gates nothing reaches
production.

## How it works

```
Scanner ──▶ Agent (Muse Spark, Responses API) ──▶ Sandbox (umbra-sandbox)
  │                │ read-only + propose_patch            │ dry-run + apply
  │                ▼                                     ▼
  │         allowlist + invariantes               Verificador determinista
  │                │                                pass ▶ propose (human approves)
  │                │                                fail ▶ rollback via snapshot
  └── finding with attribution (audit log / managedFields)
```

Agent safety: the model never applies anything; `tool_choice: auto` with the
code-enforced list (`internal/agent/tools.go`); typed patches against
allowlist; secrets redacted before the prompt (`internal/evidence`); T4
covers injection; no auto-apply; minimal read-only RBAC. Detail in
`docs/SECURITY.md` and `docs/META.md`.

## Quickstart (5 commands)

```bash
bash scripts/kind/create-cluster.sh   # 1. cluster umbra + audit log
bash scripts/build-images.sh          # 2. kind-only images
kubectl apply -k config/crd           # 3. CRDs
make bench                             # 4. deterministic scoreboard row
cd ui && npm ci && npm run build      # 5. UI
```

Or `make demo` for everything at once. Requires `MODEL_API_KEY` in env only
for model modes; **nothing leaves except the model call, with
sanitized evidence**, and `MODEL_BASE_URL` points at a local model.

## Benchmark

Three rows, same cases, seed 42, 3 repeats (`bench/cases.yaml`):

| Configuration | Shows | Status |
|---|---|---|
| Deterministic only (group A) | Coverage without AI | **18/18**, `live: true` (kind 1.37.0) |
| Traps (group T) | Reject or contain | **12/12**, `live: true` (T1–T4 ×3) |
| Model with verifiers (groups A+B) | How much AI adds with safety; B is where the model adds what deterministic can't | Unmeasured (no key in this environment) |
| Model without verifiers | Bad patches that would have applied | Unmeasured |

Groups: **A** = has deterministic fix (F01–F06 and Cron rules);
**B** = has verifier but the fix is model-only (Service without
endpoints, HPA without requests, identical probes, minimal RBAC);
**C** = diagnosis only; **T** = traps (reject or contain). Each
JSONL result stores `live: true|false`; rows measured so far are
`live: true` (local kind).

Raw data: `bench/results/*.jsonl` (deterministic, 2026-11-01) and
`bench/results/scoreboard.json`. Method and honesty rules in
`docs/BENCHMARK.md`. No model figures published until measured.

## Meta usage and limitations

Muse Spark model via Responses API with tool calls; which
model, where and why in `docs/META.md`. What it doesn't do (Helm, real
sandbox traffic, real production, Prometheus, full CronJob calendar):
`docs/LIMITATIONS.md`.

## Roadmap and license

Product roadmap (unbuilt, labeled): `ROADMAP.md`
(v0.2 Helm/GitOps, v0.3 local model + SIEM, v0.4 CronJob/GPU + vCluster,
v1.0 HA + multi-cluster). License [Apache-2.0](LICENSE). Conduct:
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), [CONTRIBUTING.md](CONTRIBUTING.md).
