# Umbra — Architecture

> This document is the design contract for the self-healing loop. It is safe to publish by design — it contains no credentials, no internal endpoints and no operational context (see [SECURITY.md](SECURITY.md) §4 for the policy that guarantees that remains true).

---

## 1. Overview

Umbra closes the remediation loop on Kubernetes configuration:

```
┌─────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   Scanner   │────▶│   Agent (LLM)    │────▶│     Sandbox      │
│ (kubectl +  │     │   diagnose +     │     │  isolated kind   │
│  linters)   │     │   propose fix    │     │  ns, egress deny │
└─────────────┘     └──────────────────┘     └────────┬─────────┘
                                                      │ patch validated?
                                              ┌───────▼─────────┐
                                              │    Verifier     │
                                              │  deterministic  │
                                              │ tests + rollback│
                                              └─────────────────┘
```

Five stages, one invariant each:

1. **Scanner** — inventory + findings; never mutates.
2. **Agent** — reasons and *proposes* tool calls; never executes anything itself.
3. **Sandbox** — contains the blast radius of every candidate patch.
4. **Verifier** — deterministic code decides pass/fail; the model's opinion is not evidence.
5. **Gate** — production changes require an explicit human approval step.

## 2. Components

### 2.1 Scanner
Read-only inventory of the target cluster:

* `kubectl get` for workload inventory (deployments, pods, services, configmaps, secrets *names only*, RBAC bindings)
* [KubeLinter](https://github.com/stackrox/kube-linter) for manifest-level lint findings
* [kube-bench](https://github.com/aquasecurity/kube-bench) for CIS-style security findings

Output: a **structured findings list** — `finding_id`, `category` (resources | reliability | security | rbac | config), `severity`, `namespace`, `resource id`, plus the tool's diagnostic text. Directory names of real clusters never leak: the scanner records namespaces, not hosts (SECURITY.md §4.b).

### 2.2 Agent (the brain)
An LLM driven through **tool calling**. The model receives one finding plus prior context and may call harness-owned tools:

| Tool | Signature (conceptual) | Purpose |
|---|---|---|
| `get_pod_status` | `(namespace, pod)` | Status, restart counts, recent events |
| `get_logs` | `(namespace, pod, tail)` | Last N log lines |
| `get_manifest` | `(namespace, resource)` | Current live YAML |
| `apply_patch` | `(namespace, yaml)` | Apply a candidate patch **inside the sandbox namespace only** |
| `run_verification` | `(namespace)` | Run the deterministic check suite |
| `rollback` | `(namespace, resource)` | Revert to the last known-good manifest |
| `propose_for_production` | `(patch)` | Queue the patch for explicit human approval |

**Contract**: the model returns `tool_calls`; the harness executes them and returns observations. The model has no shell, no network, no execution other than through these seven tools — unauthorized operations fail by construction. The loop is: *finding → reasoning → tool call → observation → … → diagnosis + patch → `apply_patch` (sandbox) → `run_verification` → `propose_for_production` or `rollback`.*

Model flexibility: the default brain is an open tool-calling model; the harness is model-agnostic and the same scoreboard must be reproducible with locally-open-weights alternatives (see [docs/PLAN.md](docs/PLAN.md) §6).

### 2.3 Sandbox
A dedicated `audit-sandbox` namespace in the same cluster:

* `NetworkPolicy` with **default-deny egress** (the sandbox cannot reach anything — external or internal):
  ```yaml
  # Publishable by design: placeholder names only (SECURITY.md §4.d)
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: sandbox-deny-egress
    namespace: audit-sandbox
  spec:
    podSelector: {}
    policyTypes: ["Egress"]
    # no egress rules at all → deny all egress
  ```
* Patch manifests are applied with the sandbox ServiceAccount, never with cluster-admin credentials.

### 2.4 Verifier
**Deterministic checks, written before any AI is involved**, per fault category:

| Category | Checks |
|---|---|
| Resources | Pod starts; no `OOMKilled`; requests/limits set and sane |
| Reliability | Liveness & readiness probes defined; probes passing; no rising restart count |
| Security | `securityContext` hardened (no root, no privileged, no `:latest` tag) |
| RBAC | Bindings after patch are **a subset** of before (no widening) |
| Config | Pod reaches `Running`; references resolve (`ImagePullBackOff` gone) |

Any failure triggers `rollback`; the "clean rollback" scoreboard column requires *no residue* — the target namespace must look exactly as before the patch, byte for byte.

### 2.5 Production gate
A patch that passes the sandbox is **proposed** (`propose_for_production`) — and then it waits. A human approves (or rejects) and only then does the harness apply the change against the target (real) namespace. There is no auto-apply path, ever, by design.

## 3. Non-goals (deliberate exclusion, per scope)

Multi-cluster/multi-tenant orchestration; GitOps reconciliation (Helm/ArgoCD); metrics-stack integration (Prometheus/Grafana — `kubectl top` + events suffice); polished dashboards; free-form chat.

## 4. Security invariants recap

Detailed rationale lives in [SECURITY.md](SECURITY.md) §8; the summary:

* least-privilege ServiceAccounts only — never `cluster-admin`
* sandbox namespace with deny-egress as the first destination of every patch
* deterministic verification, never "trust the model"
* production = explicit human approval, always
* rollback without residue, always re-measurable (scoreboard column)
