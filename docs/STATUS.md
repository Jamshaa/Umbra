# STATUS.md — la única tabla que importa

Tres columnas por función. **Vivo** = ejecutado contra kind o contra el
modelo con clave. Unit test con fixtures no cuenta como vivo. El README
enlaza aquí y no repite afirmaciones.

## Núcleo

| Función | Implementado | Unit tests | Verificado en vivo |
|---|---|---|---|
| Detectores F01–F06, F21 | Sí | Sí | F01–F06: fallos confirmados en kind (OOMKill loop F02, Pending F03); resto no |
| Detectores Cron (deadline, suspend, solape, tz, bounds, historial, GPU) | Sí | Sí | No (sin cluster) |
| Detectores grafo (Service, HPA, RBAC, Namespace) | Sí | Sí | F36 live (Service sin endpoints); F26 invertido es inadmisible en etcd → solo estático |
| Verificadores F01–F06 + Cron + grafo | Sí | Sí | F01–F06 vía sandbox (18/18 `live: true`) |
| Fixes F01–F06, Cron | Sí | Sí (Fix→Verify encadenado) | F01–F06 aplicados en sandbox y revertidos |
| Escalera de puertas + fail cerrado | Sí | Sí | Sí: dry-run + apply + rollback en `umbra-sandbox` por caso |
| Sandbox aislado (deny-all) | Sí (`config/sandbox/`) | — | Sí: BLOCKED dentro, REACHED fuera |
| Sanitizer + T4 | Sí | Sí (incl. inyección) | T4 loop exige clave |
| Recibo encadenado + `receipts verify` | Sí | Sí | No |
| Aprobación autodeclarada + `umbra-approver` | Sí | Sí | No (sin audit vivo) |
| Atribución incl. impersonación | Sí | Sí | Sí: kind registra `user` + `impersonatedUser`; `Present()` lo rotula |
| Cliente Responses API + loop + 8 tools | Sí | Sí (fake kube + scripted model) | No (sin clave) |
| `umbra-agent diagnose`, `umbra-bench` 3 modos | Sí | No (binarios; bench validado en estático) | Determinista sí; modelo no |
| Scoreboard UI + timeline live/mock rotulada | Sí | Build+lint | Datos vivos no |
| Bench A 21/18 + T 12/12 | Sí (JSONL + scoreboard) | — | `live: true` (kind 1.37.0) |
| Handpatch grupo B (4 correctos + 4 incorrectos) | Sí (`bench/handpatch/`) | — | `live: true`: correctos pasan, incorrectos rechazados con puerta y motivo |

## Complemento (código existe, evidencia viva pendiente)

| Función | Implementado | Unit tests | Verificado en vivo |
|---|---|---|---|
| PR real a GitHub | Plan-only | Sí | No (sin GH_TOKEN) |
| Minería de plantillas | Sí | Sí | No |
| Correlación cambio-incidente | Sí | Sí | No |
| Grafo de impacto | Sí | Sí | No |
| Calendario CronJob | Parcial (`NextCronRuns` + solape) | Sí | Analizado en vivo (Jobs cada minuto en punto; sleep-90 con Allow solapa 2 activos); sin harness de bench |
| `model-raw` (evaluación, no modo sombra del operador) | Sí | — | No (sin clave) |

## Diseño sin construir (ROADMAP.md)

Helm/chart, modelo local medido, SIEM, Slack/Teams, doble aprobación,
servidor MCP, vCluster, multi-cluster, métricas Prometheus, plugin kubectl,
supply-chain completa (Scorecard), envtest, fuzzing, e2e en CI, modo sombra
del operador, recuperación tras reinicio, instalación `install.yaml`.

## Notas de nombres

- `model-raw` = modo de **evaluación** del bench (qué se habría aplicado sin
  puertas). No es el modo sombra del operador (informar qué habría
  arreglado), que **no existe**.
- "Operador": el reconciliador mantiene estado y puertas; el agente
  (`umbra-agent`) propone y el bench mide. No hay autonomía de aplicación.
