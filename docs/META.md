# META.md — cómo Umbra usa los modelos de Meta

## Modelo y API

- **Modelo:** Muse Spark (`muse-spark-1.3`, configurable por `MODEL_NAME`).
- **API:** Responses API (`POST {MODEL_BASE_URL}/responses`), recomendada por
  Meta para agentes porque conserva el razonamiento entre llamadas a
  herramientas vía `previous_response_id`.
- **Cliente:** `internal/brain/brain.go` (commit `d6fabd4`, ola T1). `tool_choice`
  es siempre `"auto"`: la lista permitida la impone el código
  (`internal/agent/tools.go`), nunca el modelo. `reasoning_effort` no se envía
  jamás: Muse Spark siempre razona y `reasoning_effort: none` devuelve 400.
- **URL base configurable:** `MODEL_BASE_URL` (por defecto
  `https://api.meta.ai/v1`) admite un servidor compatible con OpenAI para la
  variante local (Muse Glimmer); ver ROADMAP.md v0.3.

## Dónde se usa

| Fichero | Papel |
|---|---|
| `internal/brain/brain.go` | Transporte: 1 round-trip, reintentos solo ante 429/5xx/fallos de red, caché por hash, `Usage` (tokens, latencia, response id) por llamada |
| `internal/agent/loop.go` | Bucle: system prompt anti-inyección, `MaxSteps` 8, presupuesto de tokens, `propose_patch` como única salida |
| `internal/agent/tools.go` | 8 herramientas: `get_manifest`, `get_events`, `get_pod_status`, `get_logs` (sanitizadas), `get_related`, `get_changes`, `get_impact` + `propose_patch` |
| `cmd/umbra-agent/main.go` | Wiring vivo: kube real → loop → puertas (`--sandbox` o solo memoria) |
| `cmd/umbra-bench/main.go` | Modos `model` (puertas puestas) y `model-raw` (puertas quitadas, solo mide) |

## Cuándo el modelo sí hace falta

Solo causas ambiguas: CrashLoop (F07), referencias rotas (F16–F18),
puertos que no coinciden (F20). F01–F06 tienen arreglo determinista y **no
pasan por el modelo** (`internal/fixes`).

## Tokens por ejecución

Medidos por el propio bench (`tokens_total` en `bench/results/scoreboard.json`).
Sin medición viva publicada aún: la tabla del README solo publicará cifras
de `bench/results/*.jsonl`. Regla anti-quema: `MaxTokensPerFinding`,
caché de turnos idénticos, y si los créditos aprietan se baja a 2
repeticiones o a menos casos (ver BENCHMARK.md).
