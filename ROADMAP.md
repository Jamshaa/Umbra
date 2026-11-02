# Umbra — Honest roadmap

Designed, not built (not presented as implemented):

## Roadmap

### v0.2 — GitOps and prevention
- **Helm**: `umbra` chart + `umbra-crds` subchart; the chart NEVER materializes secrets (only `apiKeySecretRef`).
- **CI mode**: `umbra scan ./manifests` with exit code and GitHub Action.
- **ValidatingAdmissionPolicy (CEL)**: prevent fixed fault classes (GA since K8s 1.30).
- **Generic webhooks** for Slack/Teams; Jira for the unfixable.

### v0.3 — Local model and compliance
- **Local Muse Glimmer run**: unmeasured yet; would serve via an OpenAI-compatible provider.
- **SIEM export** in JSON lines; compliance report (CIS, Pod Security).

### v0.4 — Fidelity and coverage
- **vCluster**: higher-fidelity sandbox by creating lightweight isolated clusters inside the host.
- **Image scanning** with Trivy or Grype.
- Templates for CronJobs and GPU resources.

### v1.0 — Serious production
- **Signed image + SBOM**: cosign + syft in the build pipeline.
- **Global kill switch**, change budgets per hour/namespace, freeze windows, annotation exceptions with reason and expiry.
- **Reviewer routing** by namespace owner label.
- **Multi-tenant** with per-team quotas.
- **Vault integration** for Secrets.
- Leader election, high availability, multi-cluster.

## Stated limits (not promises)

- The sandbox checks that something starts, not that it holds real traffic.
- Checks that kind networking enforces NetworkPolicy; if not, install a CNI that does.
- Muse Spark Open has 816B params; self-hosting is unrealistic; serve via a provider. Confirm the partner serves it and whether it counts for the open-weights bonus.
- Meta credits are limited: track tokens per benchmark run.
- Check the official rules whether AI use in development must be declared; write nothing false in the README.
- Benchmark figures come from the test cluster, not a production one.
- `metrics-server` for comparing against real usage isn't included in kind by default; if it doesn't install in 10 minutes, it's out.
