# AI_USE.md — how AI is used in this project

Facts derived from this repository (not marketing):

## Development workflow with agents

- This repo is worked on with an agent flow described in `AGENTS.md` and
  `docs/agents/PLAYBOOK.md` (Boss + workers, atomic tasks with observable
  acceptance criteria).
- Policy: agents never push, never open pull requests, and never merge;
  integration into the main tree is a human action. Workers commit only
  on their own task branch in their own worktree.
- Bounds on that flow: diff review against the task criteria, the
  pre-push checklist in `SECURITY.md` (including a local secrets scan),
  the test suites, and CI (build, vet, tests, lint, catalog and claims
  gates) must pass before anything is presented as done.

## How the product uses Meta models at runtime

- The agent calls Meta models through the Responses API
  (`internal/brain`), with a fixed tool list enforced by code
  (`internal/agent/tools.go`), an anti-injection system prompt, step and
  token budgets, and reads confined to the diagnosed namespace.
- The model only proposes patches. Deterministic code decides: the gate
  pipeline (allowlist, invariants, dry-run, sandbox, verifier) and human
  approval are what allow any change through. F01–F06 and the CronJob
  rules never reach the model (deterministic fixes); cases without a Go
  verifier fail closed (diagnosis only).

## Development tooling declaration

Herramientas y modelos usados en el desarrollo: <CONFIRM-BY-AUTHOR>
