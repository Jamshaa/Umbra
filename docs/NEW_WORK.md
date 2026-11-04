# NEW_WORK.md — qué se construyó durante el evento

- **Periodo del evento:** 26 oct 2026 12:00 ET (17:00 CET) a 4 nov 2026
  21:00 ET.
- **Primer commit:** `f27ef07946e17683c2971074929d0a4b578a3f64`,
  2026-10-27T21:40:47+01:00 («Day-0 baseline»). Es posterior al inicio
  del evento; no hay commits anteriores al inicio en el historial
  (`git log --reverse --format='%H %ad %an %s' --date=iso-strict`).
- **Total al escribir esto:** 95 commits, todos dentro del periodo.

## Construido por componente

- **CRDs y operador:** `api/v1alpha1` (Finding, RemediationPolicy,
  ComplianceTemplate), reconciler con máquina de estados, timeline
  auditada y saneamiento de evidencia (`internal/controller`).
- **Detección y verificación determinista:** scanner F01–F06/Cron/graph
  (`internal/scanner`), verificadores (`internal/verify`), arreglos
  deterministas (`internal/fixes`) y escalera de puertas
  (`internal/invariants`: allowlist, invariantes, dry-run, sandbox,
  verificador, rollback).
- **Agente con modelo Meta:** cliente Responses API (`internal/brain`),
  loop con 8 herramientas de solo lectura y `propose_patch`
  (`internal/agent`), binario `umbra-agent`.
- **Bench y honestidad de cifras:** catálogo `bench/cases.yaml`,
  scoreboard generado desde `bench/results/*.jsonl`, tablas renderizadas
  por `scripts/docs/render_tables.py` y `claims_lint`.
- **Recibos y atribución:** cadena de recibos HMAC (`internal/receipt`),
  `umbra receipts verify`, atribución por managedFields + audit log con
  impersonación rotulada (`umbra changes`).
- **Demo:** inyectores F01–F40 y trampas T1–T4, handpatch del grupo B,
  scripts `scripts/demo/*`, conjunto curado `bench/demo-set.yaml` y
  `demo/live/shop-api.yaml`.
- **UI:** pixel-art en `ui/` con modo LIVE (umbra-api) y mock etiquetado;
  el panel de detalle muestra el veredicto solo desde la entrada
  `gate_verdict` de la timeline.

## Terceros y uso de IA

- Componentes de terceros y licencias:
  [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).
- Uso de IA en desarrollo y en runtime: [docs/AI_USE.md](AI_USE.md).
- Uso de modelos de Meta: [docs/META.md](META.md).
