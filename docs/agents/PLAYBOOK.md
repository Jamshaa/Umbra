# Playbook del equipo Pixel — Umbra

> **Para el Jefe (boss):** este es tu libro de misiones. Las tareas de la
> Ola 1 (§4) van con payload EXACTO, directas a `create_task`. Las siguientes
> (§5) las derivas tú con la misma disciplina, leyendo `docs/PLAN.md`
> (matriz F01–F20) y `docs/ARCHITECTURE.md` antes de trocear.
> **Para workers:** ejecuta TU tarea al pie de la letra; las reglas duras del
> repo están en `AGENTS.md` (raíz): git, secretos, entorno Windows, estilo.

---

## 1. Objetivo canónico (formúlalo así)

> Operador Kubernetes Umbra funcional en local: detecta los 20 fallos
> inyectados (F01–F20 de `docs/PLAN.md`), diagnostica con LLM vía tool
> calling, aplica el parche en un sandbox aislado, verifica determinísticamente
> sin IA, y solo con verificación en verde propone el fix para aprobación
> humana explícita; rollback por snapshot sin residuo si algo falla.

Se sabe que está bien cuando: desde un cluster `kind` recién creado, los
injectors + el harness del repo reproducen la tabla del scoreboard (Día 5)
sin ningún paso manual indocumentado.

**Gates duros:**
- **Día 3:** ≥ 1 fallo curado end-to-end (detectado → verificado en sandbox →
  propuesto a producción). Si no sale: para, arregla la integración — no la idea.
- **Día 5:** ≥ 12/20 en el scoreboard. Por debajo: recorta el set al top-10 y
  presenta ESO. Cero inflación.

## 2. Cómo operas (recordatorio de tu rol + matices de este repo)

1. `list_board` antes de cada decisión: prioridades, dependencias y bloqueos
   reales, no los supuestos.
2. **Poco WIP**: 1–2 workers activos máx.; revisa en cuanto entregan.
3. `create_task` atómico: una preocupación por tarea; acceptance observable
   (comando, archivo, comportamiento); `files[]` indicado; `blockedBy` con los
   ids REALES que devuelve la herramienta.
4. `spawn_worker` del backlog desbloqueado por prioridad (P0/P1 primero).
5. Revisión: contrasta el diff con los criterios uno a uno. Si no es
   concluyente, nota de enmienda CONCRETA (archivo:linea + qué falta).
6. `request_approval` al cerrar cada **ola** (no por tarea): resumen de 3
   partes (qué se hizo / por qué / cómo se verificó). **La integración al
   árbol principal es una acción del humano** tras su aprobación — nunca la
   fuerces ni la des por hecha.
7. Ante `CONCURRENCY`/`NOT_RUNNABLE`: espera, reordena, reintenta más tarde.
   Nunca bucles de retry.
8. Al chat del humano: 3–6 líneas, ids concretos, estado real del snapshot.
9. Estas misiones son ADITIVAS a tu prompt de rol y a `AGENTS.md`: nada aquí
   afloja lo que allí está prohibido.

## 3. Leyes de este repo que NO se negocian (resumen)

- Workers: commit SOLO en su rama `task/<id>` de SU worktree; **nunca push,
  nunca PRs, nunca commits a `main` ni a la rama de arranque**; integración
  humana tras aprobación. `AGENTS.md` §1 manda.
- Secretos: cero material privado (`SECURITY.md` §3/§4); placeholders siempre
  (`example.com`, `<REPLACE-ME>`, namespaces ficticios `demo`); el kubeconfig
  local tiene contextos ajenos a Umbra: absolutamente prohibidos.
- Entorno: Windows; binarios por ruta absoluta (PATH rancio; rutas en
  `AGENTS.md` §3); `make` y `kustomize` no existen → `kubectl apply -k`;
  Kubebuilder CLI no existe → scaffold manual + CRDs vía
  `go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0`.
- Cada fallo F de la matriz sale SIEMPRE con sus tres artefactos juntos:
  injector + verificador determinista + fila ya presente en `docs/PLAN.md`.
- La verificación es determinista y NO usa IA, jamás.
- Todo lo que va a `status` de un Finding pasa antes por el sanitizador de
  evidencia (`internal/evidence`, se construye en Ola 1).

## 4. OLA 1 · Fundación Go (Día 1) — payloads EXACTOS para `create_task`

Crea las 5 en orden; encadena `blockedBy` con los ids reales que devuelva
`create_task` (la cadena real manda sobre la sugerida). Todas P0 con rol
`dev`, salvo 4.4 (P1).

### 4.1 — sin dependencias

```
title:        "CRD API v1alpha1: RemediationPolicy y Finding"
priority:     P0
requiredRole: dev
acceptance:   "go build ./..." y "go vet ./..." verdes; zz_generated.deepcopy.go
  GENERADO por controller-gen (no escrito a mano); los markers rbac de los
  tipos generan config/rbac/role.yaml con mínimo privilegio;
  nombres: remediationpolicies.umbra.io y findings.umbra.io; ambos
  cluster-scoped; cero dependencias fuera de k8s.io/api,
  k8s.io/apimachinery y sigs.k8s.io/controller-runtime.
description: |
  Módulo del operador desde cero. go.mod con module=github.com/Jamshaa/Umbra
  y la versión de Go instalada (comprueba con `go version`).

  api/v1alpha1/doc.go: doc de paquete + groupVersion umbra.io/v1alpha1.
  api/v1alpha1/groupversion_info.go: estándar controller-runtime.

  remediationpolicy_types.go — Spec:
    TargetSelector (matching por namespace/workload del objeto atendido)
    Mode           (Auto | RequireApproval)
    Escalation     (maxAutoFixes int: tras N auto-fixes fallidos la política
                    pasa a exigir aprobación humana automáticamente)
  RemediationPolicy Status: Conditions (estilo kubectl).

  finding_types.go — Spec:
    Severity         (High | Medium | Low)
    Category         (Resources | Reliability | Security | RBAC | Config)
    TargetRef        (kind/namespace/name del recurso observado)
    Detector         (string corta: qué componente lo detectó)
    Diagnosis        (string opcional: texto del LLM)
    ProposedPatch    (string YAML opcional)
    PrePatchManifest (string YAML opcional: snapshot para rollback)
    State            (Detected | Analyzing | FixProposed | Verified | Failed |
                      Reverted)
  Finding Status:
    Timeline    []TimelineEntry{Tool, Args, Result string, Ts int64}
    Conditions

  Markers rbac en ambos tipos: get/list/watch + update/patch de status +
  update de finalizers — NADA más. No scaffolds extra de kubebuilder: esta
  tarea es expresa y mínima.
files:
  - api/v1alpha1/doc.go
  - api/v1alpha1/groupversion_info.go
  - api/v1alpha1/remediationpolicy_types.go
  - api/v1alpha1/finding_types.go
  - go.mod
```

### 4.2 — bloqueada por 4.1

```
title:        "Layout kubebuilder-v4 manual: cmd/main.go y config kustomize"
priority:     P0
requiredRole: dev
acceptance:   "go build ./..." verde; controller-gen v0.19.0 (vía go run, según
  AGENTS.md) genera config/crd/bases/*.yaml desde ./api/...;
  "kubectl apply -k config/default" sobre kind-umbra crea los 2 CRDs;
  "kubectl get crd | grep umbra" los muestra; el Deployment del manager NO se
  despliega todavía (eso es Ola 2).
description: |
  Scaffold manual del layout kubebuilder v4 SIN CLI (no está instalado):

  cmd/main.go: controller-runtime manager (scheme con v1alpha1, probes y
  metrics en localhost, arranque ctx + señales de proceso).

  internal/controller/: paquete declarado listo (doc.go).
  config/: árbol kustomize fiable:
    crd/bases (generado por controller-gen) + crd/kustomization.yaml
    rbac/ (role por markers + leader election + rolebindings)
    manager/ (Deployment plantilla imagen ghcr.io/example/umbra:dev; en
      default/ queda DESHABILITADO hasta Ola 2)
    default/ (kustomization que ensambla crd+rbac+manager)

  Makefile DOCUMENTAL con objetivos estándar (manifest/generate/build/deploy)
  y el comando literal de cada uno en comentarios — make NO existe en esta
  máquina; quien ejecute debe ver la orden equivalente en el comentario.
files:
  - cmd/main.go
  - internal/controller/doc.go
  - config/crd/kustomization.yaml
  - config/rbac/kustomization.yaml
  - config/manager/kustomization.yaml
  - config/manager/manager.yaml
  - config/default/kustomization.yaml
  - Makefile
```

### 4.3 — bloqueada por 4.2

```
title:        "Reconciler RemediationPolicy: sandbox, cuota y condiciones"
priority:     P0
requiredRole: dev
acceptance: aplicar 1 RemediationPolicy de muestra en kind-umbra crea
  AUTOMÁTICAMENTE ns umbra-sandbox + NetworkPolicy deny-all ingress/egress +
  ResourceQuota con límites prudentes; Ready=true en conditions; segundo
  reconcile SIN cambios (idempotente); "go test ./internal/controller/..."
  verde con fake client (≥3 casos: creación, idempotencia, convergencia de
  cambios de spec).
description: |
  Reconciler controller-runtime estándar, idempotente. Al reconcilar una
  RemediationPolicy garantiza:
  - namespace del sandbox `umbra-sandbox`,
  - NetworkPolicy DENY-ALL (ingress+egress; sandbox SIN RED por diseño — ver
    ARCHITECTURE.md; el DNS se bloquea: nada se escapa),
  - ResourceQuota prudente (pods con techo, requests/limits en CPU/mi),
  - labels trazables.
  Conditions: Ready/Progressing legibles con kubectl. config/samples/ con una
  política viable mínima. Prohibido tocar nada fuera del sandbox y del status
  del CRD propio.
files:
  - internal/controller/remediationpolicy_controller.go
  - internal/controller/remediationpolicy_controller_test.go
  - config/samples/remediationpolicy_v1alpha1.yaml
```

### 4.4 — bloqueada por 4.1 (mejor aún, por 4.2)

```
title:        "Reconciler Finding: máquina de estados, timeline y saneo"
priority:     P1
requiredRole: dev
acceptance: cada transición válida e inválida de la máquina de estados
  cubierta por test unitario con fake client; internal/evidence/sanitize.go
  con tests que cubren: strings parecidos a tokens/API keys → <REPLACE-ME>,
  kubeconfig detectado y neutralizado, YAML estructural no tocado;
  "go test ./internal/..." verde; una TimelineEntry persiste íntegra tras
  update/patch del status.
description: |
  internal/controller/finding_controller.go: reconciler del Finding que hace
  cumplir la máquina de estados (Detected→Analyzing→FixProposed→Verified /
  Failed→Reverted), persiste TimelineEntry SOLO en status y refleja el estado
  del flujo en conditions. Todo lo que entra en status pasa ANTES por
  internal/evidence/sanitize.go (sanitizador de evidencia: cualquier string
  sospechoso de credencial/host privado sale con placeholder — ARCHITECTURE
  §evidencia). Esta tarea NO integra LLM ni sandbox: es la línea de estado
  del Finding.
files:
  - internal/controller/finding_controller.go
  - internal/controller/finding_controller_test.go
  - internal/evidence/sanitize.go
  - internal/evidence/sanitize_test.go
```

### 4.5 — sin dependencias (paralelizable con 4.4)

```
title:        "Injectores F01-F03 y helpers del namespace demo"
priority:     P0
requiredRole: dev
acceptance: cada injector ejecutado sobre kind-umbra reproduce el fallo con
  evidencia kubectl exacta (F01: dump muestra ausencia de resources; F02:
  pod OOMKilled en bucle; F03: pod Pending por requests imposibles); re-ejecutar
  cada script es seguro (idempotente); cero material privado (imágenes
  ghcr.io/example/*); script POSIX (bash) corriendo en Linux y en Git-Bash
  de Windows.
description: |
  scripts/injectors/{f01_no_limits,f02_low_memory,f03_huge_requests}.sh
  + helpers de demo: scripts/demo/up.sh crea el ns demo y limpia restos,
  scripts/demo/down.sh lo retira. Cada injector implanta EXACTAMENTE su fallo:
  F01: Deployment SIN resources (límites y requests ausentes).
  F02: memory limit 8Mi + proceso que crece de memoria en loop garantiza
       OOMKill repetido (CrashLoop de OOM).
  F03: requests cpu=64 / memory=1Ti garantiza Pending sin programar nunca.
  kubectl es el único binario externo (PATH); no llames a kind/docker; no
  toques namespaces ajenos. Un script = un fallo = nombre inequívoco.
files:
  - scripts/injectors/f01_no_limits.sh
  - scripts/injectors/f02_low_memory.sh
  - scripts/injectors/f03_huge_requests.sh
  - scripts/demo/up.sh
  - scripts/demo/down.sh
  - demo/kustomization.yaml
  - demo/f01_no_limits.yaml
  - demo/f02_low_memory.yaml
  - demo/f03_huge_requests.yaml
```

## 5. Ola 2+ — kick-offs; TÚ los derivas en tareas atómicas

Doctrine: cada fallo sale con injector + verificador determinista juntos;
verificación SIN IA siempre; scoreboard honesto (sin inflación).

| Ola | Alcance semilla (el Jefe trocea) | Bloqueada por | Gate de salida |
|---|---|---|---|
| 2 | Scanner interno: detecta F01–F05 y crea CRs Finding (evidencia saneada); verificador determinista F01–F05 (pkg `internal/verify`); injectores F04–F05; activación del Deployment del manager (Ola 1.2 lo dejó deshabilitado); arranque end-to-end en kind | Ola 1 | El sistema detecta los 5 por sí solo; sandbox existe de verdad |
| 3 | Integración LLM por tool-calling (sin secretos en el repo: config por env bloqueado por .gitignore, se provee plantilla .env.example con placeholders); tools: get_logs / get_manifest / apply_patch / run_verification, TODAS limitadas al sandbox; server-side dry-run + validator deny-list de parches (patrones prohibidos: ClusterRole/admin, webhooks, RBAC propios, tokens); rollback por snapshot (prePatchManifest); auto-fix F01 end-to-end tras Ola 2 | Ola 2 | **GATE DURO Día 3**: ≥ 1 fix correcto full-loop |
| 4 | Bucle completo con reintentos + de-escalation (2 auto-fixes fallidos → RequireApproval); fallos F06–F20 por lotes de 5 (injector+verificador+Finding); scoreboard N/20 automático con "no regression" y "rollback limpio" como condiciones; conexión UI a datos reales (React Query + WebSocket al operador; mock.js se retira); generación de los CRDs nuevos si el diseño evoluciona | Ola 3 (+humano integra) | **GATE DURO Día 5**: ≥ 12/20 |
| 5 | Helm (umbra + subchart umbra-crds; el chart JAMÁS materializa secretos — solo apiKeySecretRef); replicación open-weights documentada reproducible SIN credenciales publicadas; scoreboard reproducible desde cero; vídeo <3 min + README final | Ola 4 + humano | Repo demonstrable — Día 7–9 |

## 6. Si algo se atasca

- Worker `blocked`: lee el motivo. Contexto que falta → tarea `res` ATÓMICA.
  Tarea que resulta ser dos → trocea y relanza. Fallas ajenas al worker →
  `requeue_task` con nota quirúrgica (el qué y por qué).
- Cero "wait & retry" en bucle: si tras 2 vueltas no hay datos nuevos, escala
  al chat del humano con evidencia y opciones de decidir.
- Calidad manda: una tarea mal definida devuelve deuda. Mejor 1 tarea del
  GATE en verde que 5 tareas a medias.
- Los gates Día 3 y Día 5 se comunican al humano SIEMPRE con `request_approval`
  y evidencia: es el momento culminante de la ola.
