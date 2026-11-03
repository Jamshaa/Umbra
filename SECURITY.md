# Security Policy — Umbra

> **One rule above everything else in this repository: nothing private, ever.**
> No keys, no tokens, no kubeconfigs, no cloud credentials, no internal hostnames, no personal data. This document explains what that means precisely, how the repo enforces it, what to do before every push, and what to do the moment something slips through.

---

## 1. What Umbra is — and what that means for security

Umbra is an open-source agent that automatically **audits Kubernetes configuration, proposes fixes, applies them inside an isolated sandbox namespace, verifies them with deterministic tests, and rolls back on failure**. Nothing is proposed for production without explicit human approval.

Because the tool's whole subject matter is *cluster configuration*, this repository is a high-probability target for accidentally committed credentials. The policy below is therefore non-negotiable for every contributor.

## 2. Reporting a vulnerability

**Do not open a public GitHub issue for a security/vulnerability report.**

1. Go to the repository's **Security → Advisories → New draft advisory** (GitHub keeps this private between reporter and maintainers — this is the requested channel, no email needed, and it never exposes your identity beyond what you choose to include).
2. Include: affected area (scanner / harness / sandbox / verifier / docs), steps to reproduce, impact, and (if available) a minimal proof of concept **that contains no real credentials**.
3. Expect an acknowledgment within **72 hours**, and a status update at least every **7 days** until resolution.
4. Coordinated disclosure: we follow the standard **90-day** convention, and prefer to fix → release → publish the advisory together. Credit is given unless you prefer otherwise.

We are a small project during early development; these timelines are the goal, not a contractual SLA.

**You may automate as much as you like against your own local kind cluster.** You may **not** load-tested, fuzz or probe other people's clusters or public endpoints through Umbra, and we will not accept contributions built on data obtained that way.

## 3. Version support

| Version / branch | Supported | Notes |
|---|---|---|
| `main` (unreleased) | ✅ | The only supported state during v0 development |
| Releases ≥ v0.1.0 | ✅ | Once the first release lands (see [docs/PLAN.md](docs/PLAN.md)) |
| EOL releases | ❌ | We support only the latest release line |

## 4. PROHIBITED MATERIAL — the hard policy

The following categories must **never** enter the repository, in any file, any branch, any commit message, any issue/PR body, screenshot or log excerpt:

### 4.a Credentials and secrets

* API keys and tokens of any provider (cloud, SaaS, LLM providers included)
* OAuth client secrets, refresh tokens, session cookies
* Kubernetes **kubeconfig files**, client certificates/keys, service-account JWTs (`.kube/config`, `sa-token`, etc.)
* Cloud credentials (AWS/GCP/Azure/OVH/OCI…), SSH private keys, `known_hosts`
* TLS private certificates, P12/PFX bundles, keystore passwords
* Database connection strings with embedded passwords, webhook URLs containing tokens
* Anything resembling `AKIA…`, `sk-…`, `ghp_…`, `xoxo…`, `Bearer …` — **assume a scanner exists for the exact string you embed** (it does: see §7)

### 4.b Private operational context

* Real internal hostnames, FQDNs, IPs or IP ranges of any cluster you are not publishing on purpose
* Names of internal tools, vendors, clients or projects that identify an employer or customer
* Real node/cluster names that are not part of this repo's own kind setup
* Incident notes, network diagrams, sentry/observability URLs from real infrastructure

### 4.c Personal data

* Real email addresses, phone numbers or physical addresses — yours or anyone else's
* Content of private conversations (DMs, chats) in any form

### 4.d The placeholder convention (what IS allowed)

In every example, config, manifest and test fixture:

* Use obviously-fake values: `example.com`, `kind-umbra`, `<REPLACE-ME>`, `CHANGE_ME`
* Ship env-var *references* (`MY_TOKEN` as a name), never values — and only via a committed `.env.example`; real values belong in a local `.env` that is [.gitignore](.gitignore)-ignored
* If you need a file that resembles real infra for local testing, it must glob-match something git-ignored (e.g. `config-*.local.yaml`) — and it still should not contain *other people's* credentials

**Workflows, CI and documentation inherit this rule by default.** Every `.md` and every workflow in this repo must be safe to publish verbatim — treat that as a review requirement, not a hope.

## 5. Pre-push checklist (mandatory, every push)

1. `git status --porcelain` — know *exactly* which files leave your machine. If you cannot justify each one in one sentence, remove it.
2. `git diff` and `git diff --cached` — read the actual outgoing bytes of new files end to end.
3. Run the secrets scanner locally:
   ```bash
   # after installing gitleaks (https://gitleaks.io)
   gitleaks detect -v --source .
   ```
   (alternatively: `trufflehog filesystem .`)
4. Sanity-check the **negative** cases: `git ls-files` must show **no** `.env`, `kubeconfig*`, `*.pem`, `*.key`, `*.p12`, or `*credentials*` files.
5. Branches are pushed as ordinary branches and merged by PR. **`--force`/`git push -f` is forbidden on all shared branches** — history rewriting is a maintenance operation that requires coordination (see §6).

## 6. If a secret was committed — incident protocol

A pushed secret must be considered **compromised the moment `git push` returns success**. Publicness is irrelevant; exposure to the git remote alone is enough.

1. **Rotate/revoke the compromised credential immediately.** Deleting the file or the commit does nothing to its value — crawlers scrape public git hosts continuously and reach pushed secrets within minutes.
2. **Verify the rotation actually closed the old value** (try the leaked credential: it must now fail; on cloud providers, confirm via audit logs).
3. **Check blast radius**: for leaked kubeconfigs/cloud creds, scan audit logs / access logs for the window between the push and the revocation.
4. **Only after rotation**, coordinate a history purge (`git filter-repo` + force-push on a coordinated window, then notify every cloner). This is a maintainer-coordinated, never a unilateral, operation.
5. **Open a draft security advisory** (§2) documenting what was leaked, its category, revocation timestamps and remediation — so the next contributor learns from a real pre-push checklist miss.

## 7. Enforcement & guardrails in this repo

| Guardrail | Where | What it does |
|---|---|---|
| `gitleaks` CI scan | [`.github/workflows/security.yml`](.github/workflows/security.yml) | Scans full history (not just the diff) on every push/PR + weekly schedule; fails the build on any detected secret |
| `.gitleaks.toml` baseline | [`.gitleaks.toml`](.gitleaks.toml) | Keeps gitleaks' default rulesets ON; allowlists are the only acceptable change and must be minimal |
| `.gitignore` deny-list | [`.gitignore`](.gitignore) | Blocks the usual carriers (`.env`, `*.pem`, `kubeconfig*`, `*credentials*`, dumps, logs) from ever being staged |
| `.gitattributes` | [`.gitattributes`](.gitattributes) | Line-ending hygiene; binaries (incl. accidental binary secret stores) excluded from diffs |
| This document | — | §4 is the norm; §5 the checklist; §6 the incident runbook |

**CI is a safety net, not a substitute for reviewing your own diff** (§5 step 2). If gitleaks fails on your PR: the finding is treated as a real leak until proven otherwise — resolve per §6 first, then discuss allowlisting with maintainers.

## 8. Security posture of the tool itself

Contributors extending Umbra must respect the invariants the README promises:

* **Least privilege everywhere**: the agent's in-cluster identity uses a dedicated ServiceAccount whose RBAC covers only target namespaces' read + named sandbox namespace edit — never `cluster-admin` (that exact anti-pattern is one of the 20 fault cases.)
* **Sandboxing first**: every proposed patch lands initially in the `audit-sandbox` namespace under a default-deny egress `NetworkPolicy`. No patch may be auto-applied outside it, ever.
* **Deterministic verification, human gate**: "verification" is what deterministic code says (probes pass, no crashloops, RBAC set not expanded) — never what a model claims. Production application is always an explicit, human-approved step (`propose_for_production`), never automatic.
* **Rollback is a first-class path**: every mutation must be reversible without residue (`kubectl rollout undo` / re-apply-the-previous-manifest), and the harness must prove it (scoreboard column "clean rollback").
* **No direct execution by the model**: the LLM only returns `tool_calls`; execution is harness-owned, so unauthorized operations fail by construction.
* **Enforced hardening (audited 2026-11-02)**: approval edits constrained by admission policy ([`config/approval-policy/`](config/approval-policy)) — non-system writers may change only the `umbra.dev/approved` annotation; agent reads confined to the diagnosed namespace (code-enforced, not prompt-enforced); receipt verification requires HMAC signatures (`UMBRA_RECEIPT_HMAC_KEY`); `MODEL_BASE_URL` must be HTTPS or loopback HTTP; Finding text fields capped at 256 KiB with DNS patterns on `targetRef`; bench executes only paths under `scripts/`.

## 9. Acknowledgments

Security hygiene for this repo leans on: [Contributor Covenant v2.1](CODE_OF_CONDUCT.md), [gitleaks](https://github.com/gitleaks/gitleaks), [GitHub Security Advisories](https://docs.github.com/en/code-security/security-advisories), and the collective wisdom of every "public key on GitHub" postmortem ever written — let's not star in one.