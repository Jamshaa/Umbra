# Umbra

> **Self-healing Kubernetes configurations, end to end:
> detect → diagnose → patch → apply in a sandbox → verify → roll back if anything fails.**

**Status: early development (pre-release).** The roadmap lives in [docs/PLAN.md](docs/PLAN.md) and the design in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

<!-- Badges available to add on first public push:
- CI status (ci.yml), security scan (security.yml), License: Apache-2.0.
Purposefully not added yet: the workflows below exist, but the first real commit that lands
on `main` is what makes them run. Fake green badges before that would be dishonest. -->

## Why Umbra exists

There is no shortage of tools that *detect* Kubernetes misconfiguration. The gap is that almost none of them *close the loop* — detect, propose a fix, test it in isolation, verify it, and roll back automatically if it breaks.

| Tool | What it does | What it does **not** do |
|---|---|---|
| [KubeLinter](https://github.com/stackrox/kube-linter) | Static manifest linting | No fixes, no runtime verification |
| [kube-bench](https://github.com/aquasecurity/kube-bench) | CIS Benchmark audit | Audit only, never repairs |
| [Polaris](https://github.com/FairwindsOps/polaris) | Dashboard + admission controller + CLI | Blocks at deploy time; never repairs what is already running |
| Closed-source "AI remediation" products (e.g. AlertMend) | Autonomous scaling/remediation | Commercial, closed source, opaque |

**The gap Umbra fills:** a fully open-source agent that closes the complete loop — *audit → fix → sandbox → verify → rollback* — for **general configuration problems**, not just security findings.

## How it works

```
┌─────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   Scanner   │────▶│     Agent (LLM)  │───▶ │     Sandbox      │
│ (kubectl +  │     │  tool-calling:   │     │  (isolated kind  │
│  linters)   │     │  diagnose + fix  │     │   namespace with │
└─────────────┘     └──────────────────┘     │  egress denied)  │
                                             └────────┬─────────┘
                                                      │ patch validated
                                              ┌───────▼─────────┐
                                              │    Verifier     │
                                              │ deterministic   │
                                              │ tests + rollback│
                                              └─────────────────┘
```

| Stage | Component | Responsibility |
|---|---|---|
| 1. Scan | Scanner | Inventory the cluster (`kubectl get`, KubeLinter, kube-bench) and produce structured findings with severity. |
| 2. Diagnose & propose | Agent (LLM) | Takes one finding, gathers extra context via tools (logs, events, status), proposes a YAML patch, decides whether applying it is safe. |
| 3. Sandbox | Sandbox | Applies the patch inside an isolated namespace (`audit-sandbox`) whose NetworkPolicy forbids egress — the cluster is the blast radius limit. |
| 4. Verify | Verifier | **Deterministic code, not the LLM:** pod starts, probes pass, no CrashLoop, RBAC not widened. Anything unexpected → `rollback`. |
| 5. Gate | Production gate | Only if the sandbox check suite passes, the change is *proposed* for production and waits for explicit human approval. |

**The agent never executes anything directly.** The LLM returns `tool_calls`; the harness owns the execution:

| Tool exposed to the model | Purpose |
|---|---|
| `get_pod_status(ns, pod)` | Status, restarts, events |
| `get_logs(ns, pod, tail)` | Last N log lines |
| `get_manifest(ns, resource)` | Current YAML |
| `apply_patch(ns, yaml)` | Apply in the sandbox namespace |
| `run_verification(ns)` | Run the deterministic checks |
| `rollback(ns, resource)` | Revert the change |
| `propose_for_production(patch)` | Queue the change for explicit human approval |

## The target: 20 injected faults

Development is driven by a matrix of 20 deliberately-broken workloads. Every fault has **two artifacts written together, before any AI is involved**: an **injector script** and a **deterministic verification test**.

| Category | Example faults | Verification |
|---|---|---|
| Resources | Missing limits, unrealistic requests, OOMKilled | Pod starts, no OOM |
| Reliability | Missing liveness/readiness probes, CrashLoop | Probe passes, no restarts |
| Security | `runAsRoot`, privileged containers, `:latest` images | Correct `securityContext` |
| RBAC | ServiceAccount bound to `cluster-admin` | Least-privilege permissions only |
| Config | `ImagePullBackOff`, missing ConfigMap | Pod reaches `Running` |

## What we measure

| Metric | How it is measured |
|---|---|
| Detection | Did the scanner + agent identify the fault? (yes/no) |
| Correct diagnosis | Does the agent's root cause match the injected one? |
| Correct fix | Does the sandbox pass verification? |
| No regression | Is the rest of the namespace still healthy after the fix? |
| Clean rollback | If it failed, did it revert without residue? |

The scoreboard is a single table: N/20 across those five columns. No inflation.

## What Umbra is deliberately NOT

* ❌ Multi-cluster / multi-tenant orchestration
* ❌ GitOps (Helm charts, ArgoCD, reconciliation controllers)
* ❌ Metrics-stack integration (Prometheus/Grafana) — `kubectl top` and events suffice
* ❌ A pretty dashboard — an HTML table is enough
* ❌ A free-form chatbot

## Getting started (100% local, zero cloud)

Everything runs inside a disposable [kind](https://kind.sigs.k8s.io/) cluster. Umbra development needs **no cloud provider account and never attends to real cluster credentials** — that is deliberate.

**Prerequisites**

| Tool | Version tested | Notes |
|---|---|---|
| Docker | 29.x (Docker Desktop 4.97) | WSL2 backend on Windows |
| kind | 0.33.0 | Creates the test cluster |
| kubectl | ≥ 1.37 | Client–server skew kept ≤ 1 minor |
| Python | 3.10+ | Harness & verifier |

```bash
# 1. Bootstrap the local cluster (kind defaults to kindest/node:v1.37.0)
kind create cluster --name umbra

# 2. Confirm
kubectl cluster-info
kubectl get nodes

# 3. (coming as development lands) run the harness
#    python -m umbra scan --namespace demo
```

## Security & secrets policy — read this before your first push

> **This repository must never contain private material: no API keys, no tokens, no kubeconfigs, no cloud credentials, no internal hostnames, no personal data.** The whole policy, the pre-push checklist and the leak-incident protocol live in **[SECURITY.md](SECURITY.md)**.
>
> Automated scanning (`gitleaks`) runs in CI — [.github/workflows/security.yml](.github/workflows/security.yml) — and [CONTRIBUTING.md](CONTRIBUTING.md) documents the branch model. CI is a safety net, never a substitute for reviewing your own diff.

## Repository layout

```
Umbra/
├── .github/workflows/     # CI (lint & tests) + security (secrets scanning)
├── docs/                  # ARCHITECTURE.md (design), PLAN.md (roadmap)
├── scanner/               # (planned, Day 1) kubectl + linters inventory
├── brain/                 # (planned, Day 3) LLM tool-calling harness
├── sandbox/               # (planned, Day 1) isolated namespace + NetworkPolicy
├── verifier/              # (planned, Day 2) deterministic checks per fault
├── demo/                  # (planned, Day 6) 20 fault cases + scoreboard script
├── SECURITY.md            # Security policy + secrets rules  ← START HERE
├── CONTRIBUTING.md        # Branch model, pre-push checklist
├── CODE_OF_CONDUCT.md     # Contributor Covenant v2.1
└── LICENSE                # Apache-2.0
```

## Contributing

PRs are welcome once the first milestone lands; see [CONTRIBUTING.md](CONTRIBUTING.md) for the branch model, commit conventions and the mandatory secrets checklist.

## Code of Conduct

This project follows the [Contributor Covenant v2.1](CODE_OF_CONDUCT.md).

## License

Distributed under the [Apache License 2.0](LICENSE) — including an explicit patent grant (§3), which matters for infrastructure tooling.