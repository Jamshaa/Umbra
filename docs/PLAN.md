# Umbra — Development Plan (9 days)

> This roadmap is also the acceptance contract: each day has an exit gate, and the gates are verifiable. The plan, like the rest of the repo, must remain publishable — no private infrastructure context appears here (see [SECURITY.md](SECURITY.md)).

---

## 1. Where we are (Day 0 — done)

| Item | Status |
|---|---|
| Docker Desktop 4.97 (WSL2) installed, engine up | ✅ |
| kind 0.33.0 installed, default node image `kindest/node:v1.37.0` | ✅ |
| kubectl 1.37.1 (skew ≤ 1 minor guaranteed) | ✅ |
| Local cluster `umbra` created and Ready (control-plane ready 16s) | ✅ |
| Repo scaffolding: LICENSE (Apache-2.0), SECURITY/README/CONTRIBUTING/CoC, CI + secrets workflows | ✅ |

## 2. Roadmap

| Day | Deliverable | Exit gate |
|---|---|---|
| 1 | kind cluster + first 5 injected faults + basic scanner | Scanner detects the 5 |
| 2 | All 20 faults + one verification test per fault | Tests pass **without any AI in the loop** |
| 3 | First LLM wired through tool calling; one fault healed end-to-end | **HARD GATE:** ≥ 1 correct fix. If not, the integration is broken, not the idea — stop and fix tool calling before anything else |
| 4 | Full loop with retry + rollback | ≥ 5 faults solved |
| 5 | 20 faults, first scoreboard | **HARD GATE:** ≥ 12/20. Below that: cut the set to the 10 most reliable and present *that* — no inflation |
| 6 | Minimal UI (HTML table is enough) + open-weights replication (e.g. via Modal, **with zero credentials published**) | Scoreboard reproducible |
| 7 | Video recorded (< 3 min) + README final | Demo reproducible from the README alone |
| 8 | Polish: edge cases, cleanup, docs review | Repo presentable |
| 9 | Ship | Delivered before deadline |

## 3. The 20 faults (planned matrix)

Each fault ships **three artifacts together** (write them before touching any AI): injector script + deterministic verifier + entry in this table.

| # | Category | Fault (injected on purpose) | Verification |
|---|---|---|---|
| F01 | Resources | Deployment with **no** resource limits/requests | Limits present & sane; pod starts |
| F02 | Resources | Memory limit so low it gets `OOMKilled` in a loop | Pod starts, no `OOMKilled` |
| F03 | Resources | Requests unrealistically high → scheduling `Pending` | Pod schedules onto a node |
| F04 | Resources | No limits under bursty load → repeated OOMKill/restart timeouts | No restart escalation under load |
| F05 | Reliability | No **liveness** probe configured | Liveness probe defined & passing |
| F06 | Reliability | No **readiness** probe configured | Readiness probe defined & passing |
| F07 | Reliability | Container crashes at boot → `CrashLoopBackOff` | No crashloop; restarts stable at ≤1 |
| F08 | Reliability | Readiness endpoint mismatch (Running but never Ready) | Pod reaches Ready |
| F09 | Security | Container `runAsRoot` | `runAsNonRoot: true` effective |
| F10 | Security | `privileged: true` | Privileged mode removed |
| F11 | Security | Image tag pinned to `:latest` | Immutable/diagnostic tag present |
| F12 | Security | `allowPrivilegeEscalation` not disabled + broad capabilities | `allowPrivilegeEscalation: false`, caps dropped |
| F13 | RBAC | ServiceAccount bound to `cluster-admin` | Binding is a strict subset (least privilege) |
| F14 | RBAC | Role with wildcard `resources: ["*"]` + wildcard verbs | No wildcard permissions |
| F15 | RBAC | SA granted secret-creation it does not need | Secret writes removed |
| F16 | Config | Image tag does not exist → `ImagePullBackOff` | Pod Running; correct image resolved |
| F17 | Config | Deployment references a missing ConfigMap | References resolved; pod Running |
| F18 | Config | Deployment references a missing Secret **name** (placeholder only — no real secrets, SECURITY.md §4) | References resolved; pod Running |
| F19 | Config | Typo in container `command` → instant crash | Pod Running with intended behavior |
| F20 | Config | Probe/Service port mismatch | Probe & Service ports consistent; Ready |

## 4. Scoreboard (the metric that matters)

| Metric | Question answered |
|---|---|
| Detection | Did scanner + agent identify the fault? (yes/no) |
| Correct diagnosis | Does the agent's stated root cause match the injected fault? |
| Correct fix | Did the sandbox pass verification? |
| No regression | Is the rest of the namespace still healthy after the fix? |
| Clean rollback | On failure, revert without residue? |

Single table, N/20 verdict. Any case that fails "no regression" or "clean rollback" counts as a failure of "correct fix" too — a fix with side effects is not a fix.

## 5. Reproducibility statement

The scoreboard must be regenerated from a fresh `kind` cluster with the injector scripts alone. Out-of-the-box: `kind create cluster --name umbra` → inject faults → run harness → scoreboard table. Anything that requires a manual undocumented step is a bug in the repo, not an acceptable gap.

## 6. Model layer

The default brain is a tool-calling model. Day 6 adds an open-weights replication run so the scoreboard does not depend on one vendor's opaque serving stack. **Nothing in the runbook may require publishing credentials** — model access is configured through local env files which are `.gitignore`-blocked (see [SECURITY.md](SECURITY.md) §4/§5) and CI never prints their values.
