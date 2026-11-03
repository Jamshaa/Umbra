# DEMO.md — guion de la demo (3 minutos)

Todo lo simulado se etiqueta en pantalla. Las cifras viven solo en la
tabla generada (`docs/BENCHMARK.md`, desde `bench/results/scoreboard.json`
con `scripts/docs/render_tables.py`); este guion no lleva números escritos
a mano.

## Requisitos y arranque desde un clon limpio

- Linux o WSL2. En Windows nativo cambia solo el lanzador: usa Git Bash
  para los `.sh` y los binarios `.build/host/*.exe`.
- Docker en marcha, cluster kind `umbra` creado, contexto `kind-umbra`.
- Arranque:

```bash
bash scripts/demo/up.sh
bash scripts/build-bins.sh
npm run build:live --prefix ui
bash scripts/demo/preflight.sh
```

`preflight.sh` se detiene en el primer fallo y dice qué hacer. La UI se
sirve con `umbra-api` en `http://127.0.0.1:8082` (distintivo LIVE).

## Claves efímeras (nunca en disco)

`MODEL_API_KEY` solo en tu terminal. `UMBRA_RECEIPT_HMAC_KEY` se genera
por sesión y no se guarda jamás:

```bash
# Bash / Git Bash
export UMBRA_RECEIPT_HMAC_KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
```

```powershell
# PowerShell
$b = New-Object byte[] 32
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
$env:UMBRA_RECEIPT_HMAC_KEY = ($b | ForEach-Object { $_.ToString('x2') }) -join ''
```

«Un HMAC con clave simétrica detecta manipulación para quien posee la clave; no es una firma digital ni ofrece no repudio».

## Guion (tiempos objetivo; la duración exacta permitida no es pública: confírmala)

| Tiempo | Segmento | Qué decir | Qué no mostrar |
|---|---|---|---|
| 0:00–0:20 | Contexto | «Un operador que detecta, propone con un modelo, verifica con código y revierte.» Nodos y CRDs en pantalla. | Historial de terminal, rutas personales. |
| 0:20–0:45 | Inyección y detección | «Inyecto F01 y el escáner publica el hallazgo.» `scripts/injectors/f01_no_limits.sh` + `umbra scan --namespace demo --publish`. | Ninguna clave. |
| 0:45–1:05 | Quién lo cambió | «Cada cambio queda atribuido.» Cambio con `--as`, `audit-export.sh`, `umbra changes`. La suplantación sale rotulada. | Correos y usuarios reales. |
| 1:05–1:40 | Puertas (grupo B) | «El correcto pasa; el incorrecto lo rechaza esta puerta, con esta razón.» Handpatch F36 correcto e incorrecto. | — |
| 1:40–2:15 | Modelo | «El modelo propone; las puertas deciden.» `umbra-agent diagnose` F36 con `--sandbox --publish --receipt-chain`, y `model-raw` para enseñar qué habría pasado sin puertas. Sin clave: omitir y decir «sin medir». | La clave (teclearla con `read -rs`). |
| 2:15–2:35 | Trampas | «La inyección se contiene o se rechaza.» En modo corto se enseñan los mecanismos estáticos y las filas live pre-medidas de la tabla; `DEMO_TRAPS_LIVE=1` ejecuta los watches completos (~5 min cada uno). | Presentar un replay como vivo. |
| 2:35–2:50 | Recibo y manipulación | «El recibo es tamper-evident (HMAC).» `umbra receipts verify` + `scripts/demo/tamper.sh`: la copia manipulada falla nombrando la posición; la original pasa. | La clave HMAC. |
| 2:50–3:00 | UI y cierre | Abrir la UI (distintivo LIVE, hallazgos, scoreboard). Decir en voz alta lo que está «sin medir». | — |

## Consejos de grabación

- Terminal a 18 pt o más, 1080p, historial desactivado (`HISTFILE=` /
  borra el historial antes), pantalla limpia entre pasos.
- Teclea la clave con `read -rs` para que no quede en pantalla ni en el
  historial.
- Dos tomas como mínimo; entre tomas, `bash scripts/demo/reset.sh`.
- Graba primero la toma buena completa; los cortes se hacen después.

## Árbol de decisiones

- **Sin clave de modelo:** `--no-model`; el segmento se omite y se dice
  «sin medir». Nunca inventes tokens ni latencias.
- **Falla la red:** `--replay`. Imprime el banner fijo
  `REPLAY — resultados guardados, no una ejecución en vivo` y los
  resultados guardados. Jamás se presenta como ejecución real.
- **Falla el cluster:** `bash scripts/demo/reset.sh` y volver a empezar;
  si el cluster no levanta, `bash scripts/kind/create-cluster.sh` y
  `bash scripts/demo/up.sh`.
- **Falla una puerta en directo:** es un resultado; enséñalo con su razón
  y sigue. No repitas hasta que salga verde sin decirlo.

## Cierre

El operador guarda estado y puertas; el agente propone; ningún arreglo
llega a producción sin test determinista; ningún cambio manual queda
sin atribuir. Las cifras de la tabla son las generadas (ver
`docs/BENCHMARK.md`); lo demás está «sin medir».

<!-- umbra:bench-table:start -->
| Configuración | Grupo | Live | Resultado pass/total (N × R) | Rechazados | Omitidos | Tokens | Commit |
|---|---|---|---|---|---|---|---|
| deterministic | A | true | 21/21 (7 × 3) | 0 | 0 | 0 | a0ad509 |
| deterministic | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| deterministic | B | true | 1/1 (1 × 1) | 0 | 3 | 0 | a0ad509 |
| deterministic | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| deterministic | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| deterministic | C | false | 3/3 (1 × 3) | 0 | 42 | 0 | a0ad509 |
| deterministic | T | true | 12/12 (4 × 3) | 0 | 0 | 0 | a0ad509 |
| deterministic | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | A | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | B | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | C | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | T | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-gated | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | A | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | B | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | C | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | T | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| model-raw | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | A | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | A | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | B | true | 0/4 (4 × 1) | 0 | 0 | 0 | a0ad509 |
| handpatch | B | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | C | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | C | false | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | T | true | sin medir | sin medir | sin medir | sin medir | sin medir |
| handpatch | T | false | sin medir | sin medir | sin medir | sin medir | sin medir |
<!-- umbra:bench-table:end -->
