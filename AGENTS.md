# AGENTS.md — directrices inviolables para cualquier agente en Umbra

Este archivo lo lee **cualquier** sesión OpenCode que arranca en este repo
(Jefe Pixel, workers, QA, investigator). Son reglas duras, no sugerencias.
Nada aquí contradice a `docs/agents/PLAYBOOK.md` (el libro de misiones del
equipo) ni a `SECURITY.md` (política de Secretos); si algo lo pareciera, manda
`SECURITY.md` y luego este archivo.

## 0. Qué es Umbra

**Umbra** es un operador de Kubernetes (Go + controller-runtime) que cierra el
ciclo: detectar → diagnóstico LLM con parche propuesto → aplicar en sandbox →
verificación determinista → rollback por snapshot → aprobación humana para
producir. CRDs: `RemediationPolicy` y `Finding`. UI pixel-art en `ui/` (React).

Fuente de verdad (léelos antes de decidir nada architectural):

| Archivo | Qué define |
|---|---|
| `docs/PLAN.md` | Roadmap 9 días, matriz F01–F20 de fallos, gates duros Día 3 y Día 5, scoreboard |
| `docs/ARCHITECTURE.md` | Diseño del operador: CRDs, reconciliation, sandbox, verificación, rollback, aprobación |
| `docs/agents/PLAYBOOK.md` | Las misiones del equipo Pixel: olas de tareas, payloads exactos para `create_task` |
| `SECURITY.md` | Política de secretos (§4) y qué NO se publica jamás (§3) |
| `ui/README.md` | Contrato de diseño pixel-art de la UI (paleta, tipografía, layout) |

## 1. Reglas de git — LEER DOS VECES

- Cada worker trabaja en SU worktree (`.pixel-agents/worktrees/T-*`) en SU
  rama `task/T-*`. Ahí **sí puede conmtear** su trabajo (`git add -A && git
  commit`) antes de reportar `done` — sin commit no hay revisión.
- **Prohibido**: `git push` (a cualquier remoto), pull requests, commitear en
  `main`, commitear en la rama de arranque del repo, fusionar ramas de tarea,
  operar worktrees de otros, o tocar `.pixel-agents/` (es del servidor Pixel).
- La integración del trabajo aprobado al árbol principal es **una acción
  humana**: el humano la hace y la confirma en la interfaz. Nunca la fuerces.
- El repo tiene un único commit base (Day-0). Nada sobra: cada tarea commitea
  solo su delta en su rama.

## 2. Secretos y material prohibido

Vale lo que diga `SECURITY.md` §3/§4, reducido a tres mandatos:

1. **Nada privado entra en este repo**: claves, tokens, kubeconfigs, credenciales
   de nubes, endpoints privados, IPs/hostnames reales, datos personales. Placeholders
   (`example.com`, `<REPLACE-ME>`, namespaces ficticios `demo`) siempre.
2. **Nada privado sale del repo**: no pegues en conversaciones, tickets ni código
   contenido de tu entorno local (kubeconfig incluido). El kubeconfig local tiene
   contextos no relacionados con Umbra: **jamás** los uses, los leas ni los menciones.
   Trabaja SOLO con el contexto `kind-umbra`.
3. **Sin red sin permiso**: sin curl/wget, sin APIs locales, sin instalar
   paquetes. Dependencias: solo las que la tarea fija en texto (en Go,
   `go run <módulo>@<versión>` con la versión que indique la tarea). Jamás
   añadas dependencias no declaradas en tu tarea.

## 3. Entorno quirúrgico (Windows)

- Plataforma de ejecución: Windows con shells PowerShell. Binarios SIEMPRE por ruta
  absoluta (el PATH del shell queda rancio): `docker` está en
  `C:\Program Files\Docker\Docker\resources\bin\docker.exe`,
  `kind` en `%USERPROFILE%\go\bin\kind.exe`.
- Cluster local: `kind-umbra` (kubectl context actual). Un nodo control-plane
  v1.37.0, Ready. No lo borres; los cambios que hagas en él deben ser
  re-creables desde el repo (scripts/manifests commiteados en tu rama).
- **NO instalados**: `make`, `kustomize` (binario). Usa `kubectl apply -k`.
- **NO instalado**: Kubebuilder CLI. Scaffolda A MANO el layout de kubebuilder
  v4 (api/v1alpha1, internal/controller, cmd/main, config/*) y genera CRDs con
  `go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 <args>`.
- `go` y `kubectl` están en PATH y funcionan. Node/npm también (`node` ≥ 24).

## 4. Estilo de trabajo del operador (Go)

- Controller-runtime estándar: reconcilers idempotentes, `reconcile` baratos,
  condiciones (`Conditions`) como estado observable, finalizers solo si los
  pide la tarea.
- RBAC markers en los tipos, mínimo privilegio (ver ARCHITECTURE.md §roles).
- Toda escritura a `status` de un Finding pasa por el sanitizador de evidencia
  (ARCHITECTURE.md §evidence) — nunca escribas archivos bajo status sin él.
- Los health/metrics endpoints del manager y el deployment en `config/manager`
  solo se materializan si la tarea lo pide explícitamente.

## 5. Estilo de la UI (pixel-art, NO se suaviza)

El contrato íntegro está en `ui/README.md` y `ui/src/styles/base.css`. Resumen
obligatorio: 8 colores exactos, Press Start 2P SOLO en múltiplos de 8px,
borde duro `4px 4px 0`, sombras sin blur, grid de 8px, transiciones 0ms, sin
`border-radius`, sin Tailwind ni librerías de componentes. Si algo de UI no
encaja en este contrato, la tarea está mal especificada — reporta `blocked`.

## 6. Definición de hecho (para TODAS las tareas)

1. Todos los criterios de aceptación de la tarea verificados uno a uno, con el
   comando y su salida evidenciada en la respuesta.
2. Go: `go build ./...` y `go vet ./...` en verde desde la raíz del módulo.
   UI: `npm run build` en verde desde `ui/`.
3. Sin TODOs, sin código muerto, sin placeholders “por ahora” (los placeholders
   DOCUMENTADOS del contrato de seguridad sí son válidos).
4. Sin archivos fuera del alcance de la tarea (ni “de camino”).
5. Resumen en 3 partes: qué se hizo / por qué / cómo se verificó (evidencia).

Si no puedes verificar algo: dilo en el resumen. La evidencia es tu
credibilidad — un `done` sin prueba es un `blocked` no declarado.
