# Contributing to Umbra

Thanks for helping build the missing piece of K8s remediation: a tool that closes the loop — detect, propose, sandbox, verify, roll back.

**Read [SECURITY.md](SECURITY.md) first.** Whether you are submitting one YAML or a whole subsystem, the rule is the same: *nothing private, ever* — no keys, no tokens, no kubeconfigs, no internal hostnames. Contributors' hygiene protocols and the repo's enforcement tooling are all described there.

---

## 1. Development environment

| Tool | Tested with | Why |
|---|---|---|
| Docker | Docker Desktop 4.97 (WSL2 backend, Windows) | Runs the kind nodes |
| [kind](https://kind.sigs.k8s.io/) | 0.33.0 | The local test cluster |
| kubectl | 1.37.x | Keep client/server skew ≤ 1 minor (kind 0.33.0 → node image v1.37.0) |
| Python | ≥ 3.10 | Harness, scanner glue and verifier |
| [gitleaks](https://github.com/gitleaks/gitleaks) | latest | Pre-push secrets scan (SECURITY.md §5) |

Bootstrap the dev cluster:

```bash
kind create cluster --name umbra
kubectl cluster-info
# ...work...
kind delete cluster --name umbra   # clusters are disposable by design
```

## 2. Branch model

* **`main` is the trunk**: only via pull requests; direct pushes are not part of this workflow.
* Branch off fresh `main`, keep branches short-lived (aim ≤ 1 week), one concern per branch.
* Naming (kebab-case, prefix by intent):

| Prefix | Use for | Example |
|---|---|---|
| `feat/` | New capability | `feat/sandbox-networkpolicy` |
| `fix/` | Bug fix | `fix/verifier-crashloop-check` |
| `docs/` | Docs only | `docs/metrics-table` |
| `refactor/` | No behavior change | `refactor/scanner-findings` |
| `test/` | Tests & fixtures | `test/fault-matrix-resources` |

* **Force-pushing (`git push -f`) is forbidden on shared branches.** Rewriting history is a coordinated operation — see [SECURITY.md](SECURITY.md) §6 for when it is legitimately needed and how.

## 3. Commits — Conventional Commits

Small, single-concern commits with an imperative summary:

| Type | Example |
|---|---|
| `feat` | `feat(brain): expose get_manifest tool to the model` |
| `fix` | `fix(verifier): treat first-start restartCount=0 as success` |
| `docs` | `docs(plan): add day-5 cut criteria to scoreboard` |
| `refactor` | `refactor(scanner): split kube-bench parser` |
| `test` | `test(faults): add injector for missing resource limits` |
| `chore` | `chore(ci): pin gitleaks-action version` |

## 4. Pull requests

1. Open from your topic branch into `main`.
2. The PR description states: what changes, why, how it was tested, and **which fault-matrix case (if any) it advances**.
3. CI must be green: both workflows (pair? see `.github/workflows/ci.yml` and `security.yml`).
4. Any secrets-scan failure is treated as a **real leak until proven otherwise** — rotate first (SECURITY.md §6), discuss allowlists later.
5. One maintainer approval merges.

## 5. Test philosophy (non-negotiable)

* Every injected fault ships **three artifacts together**: injector script + deterministic verifier + README-table entry. *"Both of these are written before touching the AI."*
* The verifier is **deterministic code, not model judgment**: it asserts pod phase, probe status, restart counts and RBAC deltas with plain assertions.
* Agent-quality PRs must move the scoreboard ([docs/PLAN.md](docs/PLAN.md) §4) without regressing others: a fix that helps one fault at the cost of a regression elsewhere is a net loss.
* The scoreboard generation script must be reproducible from a fresh `kind` cluster — no shading, no cherry-picking.

## 6. Pre-push checklist (mandatory — every single push)

Condensed from [SECURITY.md](SECURITY.md) §5 — read it once in full, then run these before every push:

1. **`git status --porcelain`** — justify every outgoing file in one sentence.
2. **`git diff` / `git diff --cached`** — read the outgoing bytes, especially new files.
3. **`gitleaks detect -v --source .`** — locally, not just in CI.
4. `git ls-files` shows **no** `.env`, `kubeconfig*`, `*.pem`, `*.key`, `*.p12`, `*credentials*`.
5. Examples use fake values only (`example.com`, `<REPLACE-ME>`, `kind-umbra`) — real-looking endpoints/IPs/names get flagged in review even if they are "harmless".

## 7. Code style

* Python: formatted and linted with **ruff**; docstrings on every harness/tool function (they document the model-facing contract).
* YAML (manifests/CI): 2-space indent, quotes consistent, comments explain *intent*, not syntax.
* Manifests produced by the agent always keep the source cluster's labels and get a provenance annotation; anything else is treated as tampering by the verifier.

## 8. Where to start

Good-first-contribution targets are tracked in the roadmap: [docs/PLAN.md](docs/PLAN.md). When in doubt, open a normal (public) issue describing the gap you see — and remember §2 of SECURITY.md: security-sensitive findings go through private advisories, never public issues.