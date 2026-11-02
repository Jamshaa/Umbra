# AGENTS.md — inviolable guidelines for any agent in Umbra

This file is read by **any** OpenCode session starting in this repo
(Boss Pixel, workers, QA, investigator). These are hard rules, not suggestions.
Nothing here contradicts `docs/agents/PLAYBOOK.md` (the team's mission book)
or `SECURITY.md` (Secrets policy); if anything seemed to, `SECURITY.md` wins,
then this file.

## 0. What Umbra is

**Umbra** is a Kubernetes operator (Go + controller-runtime) closing the
loop: detect → LLM diagnosis with proposed patch → apply in sandbox →
deterministic verification → snapshot rollback → human approval for
production. CRDs: `RemediationPolicy` and `Finding`. Pixel-art UI in `ui/` (React).

Source of truth (read them before deciding anything architectural):

| File | What it defines |
|---|---|
| `docs/PLAN.md` | 9-day roadmap, F01–F20 failure matrix, hard gates Day 3 and Day 5, scoreboard |
| `docs/ARCHITECTURE.md` | Operator design: CRDs, reconciliation, sandbox, verification, rollback, approval |
| `docs/agents/PLAYBOOK.md` | Pixel team missions: task waves, exact payloads for `create_task` |
| `SECURITY.md` | Secrets policy (§4) and what is NEVER published (§3) |
| `ui/README.md` | Pixel-art design contract for the UI (palette, typography, layout) |

## 1. Git rules — READ TWICE

- Each worker works in THEIR worktree (`.pixel-agents/worktrees/T-*`) on THEIR
  branch `task/T-*`. There they **may commit** their work (`git add -A && git
  commit`) before reporting `done` — no commit, no review.
- **Forbidden**: `git push` (to any remote), pull requests, committing to
  `main`, committing to the repo's boot branch, merging task branches,
  operating others' worktrees, or touching `.pixel-agents/` (it belongs to the Pixel server).
- Integrating approved work into the main tree is **a human
  action**: the human does it and confirms it in the interface. Never force it.
- The repo has a single base commit (Day-0). Nothing is spare: each task commits
  only its delta on its branch.

## 2. Secrets and forbidden material

What `SECURITY.md` §3/§4 says, reduced to three mandates:

1. **No private material enters this repo**: keys, tokens, kubeconfigs, cloud
   credentials, private endpoints, real IPs/hostnames, personal data. Placeholders
   (`example.com`, `<REPLACE-ME>`, fictitious `demo` namespaces) always.
2. **No private material leaves the repo**: don't paste into conversations, tickets, or code
   anything from your local environment (kubeconfig included). The local kubeconfig has
   contexts unrelated to Umbra: **never** use, read, or mention them.
   Work ONLY with the `kind-umbra` context.
3. **No network without permission**: no curl/wget, no local APIs, no installing
   packages. Dependencies: only those the task pins in writing (in Go,
   `go run <module>@<version>` with the version the task states). Never
   add undeclared dependencies to your task.

## 3. Surgical environment (Windows)

- Execution platform: Windows with PowerShell shells. Binaries ALWAYS by absolute
  path (the shell PATH goes stale): `docker` is at
  `C:\Program Files\Docker\Docker\resources\bin\docker.exe`,
  `kind` at `%USERPROFILE%\go\bin\kind.exe`.
- Local cluster: `kind-umbra` (current kubectl context). One control-plane node
  v1.37.0, Ready. Don't delete it; changes you make to it must be
  re-creatable from the repo (scripts/manifests committed on your branch).
- **NOT installed**: `make`, `kustomize` (binary). Use `kubectl apply -k`.
- **NOT installed**: Kubebuilder CLI. Scaffold BY HAND the kubebuilder
  v4 layout (api/v1alpha1, internal/controller, cmd/main, config/*) and generate CRDs with
  `go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 <args>`.
- `go` and `kubectl` are on PATH and work. Node/npm too (`node` ≥ 24).

## 4. Operator work style (Go)

- Standard controller-runtime: idempotent reconcilers, cheap `reconcile`,
  conditions (`Conditions`) as observable state, finalizers only if
  the task asks for them.
- RBAC markers on the types, least privilege (see ARCHITECTURE.md §roles).
- Every write to a Finding's `status` goes through the evidence sanitizer
  (ARCHITECTURE.md §evidence) — never write files under status without it.
- The manager's health/metrics endpoints and the deployment in `config/manager`
  only materialize if the task explicitly asks for them.

## 5. UI style (pixel-art, NOT softened)

The full contract is in `ui/README.md` and `ui/src/styles/base.css`. Summary:
8 exact colors, Press Start 2P ONLY in multiples of 8px,
hard border `4px 4px 0`, blur-free shadows, 8px grid, 0ms transitions, no
`border-radius`, no Tailwind or component libraries. If any UI doesn't
fit this contract, the task is misspecified — report `blocked`.

## 6. Definition of done (for ALL tasks)

1. Every task acceptance criterion verified one by one, with the
   command and its output evidenced in the response.
2. Go: `go build ./...` and `go vet ./...` green from the module root.
   UI: `npm run build` green from `ui/`.
3. No TODOs, no dead code, no "for now" placeholders (the DOCUMENTED
   security-contract placeholders are still valid).
4. No files outside the task scope (no drive-bys).
5. 3-part summary: what was done / why / how it was verified (evidence).

If you can't verify something: say so in the summary. Evidence is your
credibility — a `done` without proof is an undeclared `blocked`.
