# DEMO.md — guion y comandos (salida esperada incluida)

Duración objetivo: 3 minutos. Todo lo simulado va rotulado en pantalla.

## 0. Preparar (una vez)

```bash
make demo
```

Crea el cluster `umbra` (contexto `kind-umbra`, audit log activado),
construye y carga imágenes, aplica CRDs, levanta fixtures F01/F05/F06 y
construye la UI. ~10 min en máquina limpia.

## 1. El problema (30 s)

```bash
kubectl get deployment f01-no-limits -n demo -o yaml | grep -A3 resources
# (sin salida: no hay bloque resources)
```

"Un cambio manual sin requests/limits. Nada lo atribuye hasta que el
audit log dice quién (`docs/kind-audit.md`)."

## 2. El modelo propone (45 s)

```bash
export MODEL_API_KEY=<tu-clave>   # nunca en un archivo
go run ./cmd/umbra-agent --kind Deployment --name f07-crashloop \
  --namespace demo --fault F07
```

Salida esperada: JSON con `proposal.root_cause`, `gate_step: ok`,
`gate_pass: true`, `tokens`, `latency_ms`. La propuesta **no se aplica**:
sin `--sandbox` las puertas corren en memoria.

## 3. Verificadores y reversión (30 s)

```bash
go run ./cmd/umbra-bench --mode deterministic --cases F01,F02,F05,F06
# deterministic cases=12 pass=12 fail=0 ...
```

Cada fila de `bench/results/*.jsonl` guarda commit, modelo, fecha, semilla.

## 4. Una trampa bloqueada (30 s)

```bash
go run ./cmd/umbra-bench --mode model-raw --cases T1,T4
# T1: would_refuse=true (RBAC). T4: fail/contained si el parche no pide humano.
```

## 5. Scoreboard y cierre (45 s)

Abrir la UI: tabla por configuración y grupo (`deterministic · A`, `T`, ...) con
marca vivo/estático + timeline. **Si la timeline dice "simulación", decirlo en
voz alta.** Cierre: el operador mantiene el estado y las puertas, el agente es
el que propone; ningún arreglo llega a producción sin prueba determinista;
ningún cambio manual queda sin atribuir.
