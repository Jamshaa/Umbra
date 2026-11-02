# DEMO.md — script and commands (expected output included)

Target length: 3 minutes. Everything simulated is labeled on screen.

## 0. Setup (once)

```bash
make demo
```

Creates the `umbra` cluster (`kind-umbra` context, audit log on),
builds and loads images, applies CRDs, brings up F01/F05/F06 fixtures and
builds the UI. ~10 min on a clean machine.

## 1. The problem (30s)

```bash
kubectl get deployment f01-no-limits -n demo -o yaml | grep -A3 resources
# (sin salida: no hay bloque resources)
```

"A manual change without requests/limits. Nothing attributes it until the
audit log says who (`docs/kind-audit.md`)."

## 2. The model proposes (45s)

```bash
export MODEL_API_KEY=<your-key>   # never in a file
go run ./cmd/umbra-agent --kind Deployment --name f07-crashloop \
  --namespace demo --fault F07
```

Expected output: JSON with `proposal.root_cause`, `gate_step: ok`,
`gate_pass: true`, `tokens`, `latency_ms`. The proposal is **not applied**:
without `--sandbox` gates run in memory.

## 3. Verifiers and rollback (30s)

```bash
go run ./cmd/umbra-bench --mode deterministic --cases F01,F02,F05,F06
# deterministic cases=12 pass=12 fail=0 ...
```

Each row in `bench/results/*.jsonl` stores commit, model, date, seed.

## 4. A blocked trap (30s)

```bash
go run ./cmd/umbra-bench --mode model-raw --cases T1,T4
# T1: would_refuse=true (RBAC). T4: fail/contain unless the patch asks for a human.
```

## 5. Scoreboard and close (45s)

Open the UI: table by config and group (`deterministic · A`, `T`, ...) with
live/static mark + timeline. **If the timeline says "simulation", say so out
loud.** Close: the operator holds state and gates, the agent
proposes; no fix reaches production without a deterministic test;
no manual change goes unattributed.
