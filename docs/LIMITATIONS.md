# LIMITATIONS.md — what Umbra doesn't do (nor pretends to)

- El avance de estado lo hace el operador dentro del cluster; no está cableado en esta demo.
  Observado en código, no en ejecución: `cmd/umbra-agent/publish.go:225-230`
  sí intenta mover `spec.state` paso a paso con `Update`, y la VAP
  (`config/approval-policy/validating-policy.yaml:29`) lo deniega a
  usuarios no `system:*`. La denegación no se ignora: devuelve error,
  `publishDiagnosis` aborta antes de escribir el status
  (`publish.go:236`), y `cmd/umbra-agent/main.go` lo deja en el JSON de
  salida como `"publish": "publish failed: move finding <nombre> to
  Analyzing: <error de admisión>"`, sin cambiar el veredicto ni el código
  de salida. El Finding creado queda en `Detected` y la timeline de esa
  publicación no se persiste.
- **Red del sandbox (N0).** Hay NetworkPolicy deny-all declarada, pero su
  aplicación depende del CNI; resultado medido: sin medir en esta sesión
  (cluster no tocado por instrucción del autor), variante N0. En N0 la
  demo no afirma «aislado» ni «red denegada»: dice «separate sandbox
  namespace with quota, copy and rollback». `preflight.sh` mide y escribe
  el resultado literal en `.demo/network.txt`.

- **No Helm, no real sandbox traffic.** The sandbox verifies that the
  workload starts and passes its check, not that it holds real traffic.
  Anything requiring traffic replay is out of catalog (F04 verifies via
  limits + stable restarts, not real load).
- **Few model cases, reported as counts.** The model is evaluated on a
  small case set; results are reported as counts (pass/total), never as
  percentages without their denominator.
- **No external calls from fonts or UI.** Fonts are self-hosted and the
  UI only talks to its same-origin API; no external requests are made.
- **No real production.** Manager/production only proposes; applying is a
  human act (`umbra approve` / annotation). Never auto-apply.
- **No Prometheus.** Comparing requests against historical usage needs time
  series; `metrics-server` is a snapshot and decides nothing. Cost
  optimization goes to the roadmap.
- **Partial CronJob calendar.** `NextCronRuns` only expands `*`, `*/k` and
  numbers in minute/hour; the rest is "unknown", never healthy.
- **Static detection, not runtime.** Real CronJob overlaps, QoS under
  pressure, or multi-CronJob schedule collisions need live observation;
  the detector flags the risk, the event confirms it.
- **Unused roles.** Requires real audit log with history; out of scope.
- **Local model.** Glimmer via OpenAI-compatible endpoint: wired
  (`MODEL_BASE_URL`), unmeasured.
- **Egress.** The cluster only needs to reach the model API;
  today the agent runs on the host, so this doesn't affect images.
  If the agent moves into the manager: `FROM scratch` ships no root
  certificates and the HTTPS call would fail — copy the CA bundle and
  open egress in its NetworkPolicy (see comment in
  `Dockerfile.manager`).
