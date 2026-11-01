# Umbra

> **Self-healing Kubernetes configurations, end to end:
> detect → diagnose → patch → apply in a sandbox → verify → roll back if anything fails.**

**Status: pre-release.** Ningún arreglo llega a producción sin prueba determinista; ningún cambio manual queda sin atribuir.

> Estado función por función (implementado / unit / vivo): [`docs/STATUS.md`](docs/STATUS.md). El README no afirma nada que esa tabla no respalde.

> Captura de la UI: pendiente de la primera demo grabada (ver `docs/DEMO.md`).

## El problema

Los equipos de plataforma viven con errores de configuración: el 78% de los
encuestados de Red Hat reportó fallos de configuración en el último año. El
flujo real es siempre el mismo: algo se rompe, la primera pregunta es
"¿qué cambió justo antes?", y el arreglo manual que sigue no deja rastro
auditable. Umbra encaja en ese día a día: el hallazgo llega con quién lo
cambió, la propuesta con su prueba, y la aplicación con recibo y reversión.

## Qué lo diferencia

| Herramienta | Hace | No hace (Umbra sí) |
|---|---|---|
| K8sGPT | Escanea y explica; remediación automática alfa y limitada | Probar el parche antes de aplicarlo |
| HolmesGPT | Investiga en solo lectura | Proponer + verificar + recibo |
| Aurora | Abre PRs solo tras aprobación humana | Puertas deterministas y rollback por snapshot |

Tesis: el modelo propone, el código verifica, un humano aprueba y todo deja
un recibo. División honesta del trabajo: **el operador mantiene el estado y
las puertas; el agente es el que propone**. No es un operador autónomo: sin
propuesta del agente no hay nada que verificar, y sin puertas nada llega a
producción.

## Cómo funciona

```
Scanner ──▶ Agent (Muse Spark, Responses API) ──▶ Sandbox (umbra-sandbox)
  │                │ read-only + propose_patch            │ dry-run + apply
  │                ▼                                     ▼
  │         allowlist + invariantes               Verificador determinista
  │                │                                pass ▶ propone (humano aprueba)
  │                │                                fail ▶ rollback por snapshot
  └── hallazgo con atribución (audit log / managedFields)
```

Seguridad del agente: el modelo nunca aplica nada; `tool_choice: auto` con la
lista impuesta en código (`internal/agent/tools.go`); parches tipados contra
allowlist; secretos redactados antes del prompt (`internal/evidence`); T4
prueba la inyección; sin auto-apply; RBAC mínimo de solo lectura. Detalle en
`docs/SECURITY.md` y `docs/META.md`.

## Inicio rápido (5 comandos)

```bash
bash scripts/kind/create-cluster.sh   # 1. cluster umbra + audit log
bash scripts/build-images.sh          # 2. imágenes kind-only
kubectl apply -k config/crd           # 3. CRDs
make bench                             # 4. fila determinista del scoreboard
cd ui && npm ci && npm run build      # 5. UI
```

O `make demo` para todo de una vez. Requiere `MODEL_API_KEY` en entorno solo
para los modos con modelo; **ningún dato sale salvo la llamada al modelo, con
evidencia sanitizada**, y `MODEL_BASE_URL` permite apuntar a un modelo local.

## Benchmark

Tres filas, mismos casos, semilla 42, 3 repeticiones (`bench/cases.yaml`):

| Configuración | Qué demuestra | Estado |
|---|---|---|
| Solo deterministas (grupo A) | Cobertura sin IA | **18/18**, `live: true` (kind 1.37.0) |
| Trampas (grupo T) | Rechazar o contener | **12/12**, `live: true` (T1–T4 ×3) |
| Modelo con verificadores (grupos A+B) | Cuánto añade la IA con seguridad; B es donde el modelo aporta lo que lo determinista no puede | Sin medir (sin clave en este entorno) |
| Modelo sin verificadores | Parches malos que se habrían aplicado | Sin medir |

Grupos: **A** = hay arreglo determinista (F01–F06 y reglas Cron);
**B** = hay verificador pero el arreglo es solo del modelo (Service sin
endpoints, HPA sin requests, sondas idénticas, RBAC mínimo);
**C** = solo diagnóstico; **T** = trampas (rechazar o contener). Cada
resultado JSONL guarda `live: true|false`; las filas medidas hasta hoy son
`live: true` (kind local).

Datos crudos: `bench/results/*.jsonl` (el determinista, del 2026-11-01) y
`bench/results/scoreboard.json`. Método y reglas de honestidad en
`docs/BENCHMARK.md`. Ninguna cifra de modelo se publica hasta medirse.

## Uso de Meta y limitaciones

Modelo Muse Spark por Responses API con llamadas a herramientas; detalle de
qué modelo, dónde y por qué en `docs/META.md`. Lo que no hace (Helm, tráfico
real en sandbox, producción real, Prometheus, calendario CronJob completo):
`docs/LIMITATIONS.md`.

## Roadmap y licencia

Hoja de ruta del producto (no construido, rotulado): `ROADMAP.md`
(v0.2 Helm/GitOps, v0.3 modelo local + SIEM, v0.4 CronJob/GPU + vCluster,
v1.0 HA + multi-cluster). Licencia [Apache-2.0](LICENSE). Convivencia:
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md), [CONTRIBUTING.md](CONTRIBUTING.md).
