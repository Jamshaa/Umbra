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

`preflight.sh` se detiene en el primer fallo y dice qué hacer. Excepción:
la red es un aviso, no un fallo — escribe la variante en `.demo/network.txt`
(N1 solo si todas las sondas muestran aislamiento; si no, N0) y termina
con código 0. En N0 no se dice «aislado» ni «red denegada»: se dice
«separate sandbox namespace with quota, copy and rollback». La UI se
sirve con `umbra-api` en `http://127.0.0.1:8082` (distintivo LIVE).

Estado al preparar este guion: sin `MODEL_API_KEY`, sin toolchain Go y
sin tocar el cluster, el ensayo en vivo queda pendiente. Sin clave, el
acto A está **BLOQUEADO: no hay demo completa**.

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

## Actos A–G (comandos reales; inventario en `.demo/inventory.txt`)

Conjunto curado en `bench/demo-set.yaml` (todo lo demás: `demo: false`).
Prohibidos en demo: F17 y F32–F34 (no autocontenidos). Tiempos del
ensayo: sin medir (ensayo en vivo pendiente; se rellenan desde el log de
`scripts/demo/rehearse.sh`, fuente «reloj de rehearse.sh» o `latency_ms`).

| Acto | Comandos | Resultado esperado |
|---|---|---|
| A (F36 + modelo) | `bash scripts/demo/break.sh f36` · `.build/host/umbra scan --namespace demo --publish` · `.build/host/umbra-agent --kind Service --name f36-no-endpoints --namespace demo --fault F36 --sandbox --publish --receipt-chain .demo/receipts.chain` | tool calls, propuesta, puertas y veredicto `gate_verdict` en la timeline; recibo. Si el parche no pasa, se enseña FAIL con su razón. |
| B (F02 sin modelo) | `bash scripts/demo/break.sh f02` · `.build/host/umbra-bench --mode deterministic --cases F02 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json` · `bash scripts/demo/heal.sh` | propuesta verificada en sandbox sobre `shop-api` (respaldo F01: `break.sh f01`). |
| C (atribución) | `kubectl --context kind-umbra --as=alice@example.com -n demo scale deploy/shop-api --replicas=2` · `bash scripts/demo/audit-export.sh` · `.build/host/umbra changes --kind Deployment --name shop-api --namespace demo --audit-file .demo/audit.log` · escalar de vuelta a 1 | el cambio queda atribuido; una cuenta de servicio se presenta como cliente, nunca como persona. |
| D (puerta + model-raw) | `umbra-bench --mode handpatch --hp-kind Service` con `bench/handpatch/F36/incorrect.yaml` y `correct.yaml` (`--hp-case F36 --hp-rule ServiceEndpoints`) · `.build/host/umbra-bench --mode model-raw --cases F36 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json` | Sin versión «bien»: ningún parche F36 pasa hoy todas las puertas. Medido estático: el malo cae en `allowlist` (`/metadata/labels`), el «correcto» en `invariants` (`spec.selector changed (immutable)`) y un alternativo sin tocar el selector (etiquetas del Deployment) en `allowlist` (`/spec/template/metadata/labels/app`). Se enseñan las razones y model-raw; no se afirma ningún PASS. |
| E (trampas) | `.build/host/umbra-bench --mode deterministic --cases T2 --live --out .demo/bench --scoreboard-out .demo/scoreboard.json` · `bash scripts/demo/show-saved.sh` | T2 rechazado en dry-run en directo; T1/T3/T4 como «resultado guardado (fecha, commit)». No se ejecutan T1/T3/T4 en directo. |
| F (recibos) | `.build/host/umbra receipts verify .demo/receipts.chain` · `bash scripts/demo/tamper.sh` | la copia manipulada falla; la original pasa. Tamper evidence (HMAC), not a digital signature. |
| G (UI) | `npm run build:live --prefix ui` y abrir la UI LIVE | el detalle lee solo `gate_verdict`: «gates: PASSED/FAILED», o «gates: not recorded; state is maintained by the in-cluster operator (not wired in this demo)». Nunca deriva «verified». |

Nota de nombres reales: `umbra-agent` no lleva la palabra `diagnose`
(parsea flags desde argv[1]); `--repeats` sí existe en `umbra-bench`.

Ensayo: `bash scripts/demo/rehearse.sh --fast` (×2), una pasada final
con pausas, y una desde estado sucio solo si algo falló. Higiene:
`source scripts/demo/camera-env.sh` antes de grabar y
`bash scripts/demo/leakcheck.sh` sobre el log de la pasada final.

## Consejos de grabación

- Terminal a 18 pt o más, 1080p, historial desactivado (`HISTFILE=` /
  borra el historial antes), pantalla limpia entre pasos.
- Teclea la clave con `read -rs` para que no quede en pantalla ni en el
  historial.
- Dos tomas como mínimo; entre tomas, `bash scripts/demo/reset.sh`.
- Graba primero la toma buena completa; los cortes se hacen después.

## Árbol de decisiones

- **Sin clave de modelo:** acto A BLOQUEADO; no hay demo completa. Se
  prepara el resto y se dice «sin medir». Nunca inventes tokens ni
  latencias.
- **Modo replay:** `bash scripts/demo/run.sh --replay`. Imprime el banner
  fijo `REPLAY — resultados guardados, no una ejecución en vivo` y los
  resultados guardados. Jamás se presenta como ejecución real.
- **Red N0:** no se afirma aislamiento; se dice «separate sandbox
  namespace with quota, copy and rollback».
- **Estado sucio:** `bash scripts/demo/reset.sh` + `bash scripts/demo/heal.sh`
  y volver a empezar.
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
