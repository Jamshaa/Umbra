# Umbra — Roadmap honesto

Diseno sin construir (no se presenta como implementado):

## Hoja de ruta

### v0.2 — GitOps y prevencion
- **Helm**: chart `umbra` + subchart `umbra-crds`; el chart JAMAS materializa secretos (solo `apiKeySecretRef`).
- **Modo CI**: `umbra scan ./manifests` con codigo de salida y GitHub Action.
- **ValidatingAdmissionPolicy (CEL)**: prevencion de clases de fallos arreglados (GA desde K8s 1.30).
- **Webhooks** genericos para Slack/Teams; Jira para lo no arreglable.

### v0.3 — Modelo local y cumplimiento
- **Ejecucion de Muse Glimmer local**: sin medir todavia; serviria por un proveedor compatible con OpenAI.
- **Exportacion a SIEM** en JSON lines; informe de cumplimiento (CIS, Pod Security).

### v0.4 — Fidelidad y cobertura
- **vCluster**: sandbox con mayor fidelidad creando clusters ligeros aislados dentro del host.
- **Analisis de imagenes** con Trivy o Grype.
- Plantillas para CronJobs y recursos de GPU.

### v1.0 — Produccion seria
- **Imagen firmada + SBOM**: cosign + syft en el pipeline de build.
- **Kill switch global**, presupuestos de cambios por hora/namespace, ventanas de congelacion, excepciones por anotacion con motivo y caducidad.
- **Enrutado de revisores** segun la etiqueta de propietario del namespace.
- **Multi-tenant** con cuotas por equipo.
- **Integracion con Vault** para Secrets.
- Eleccion de lider, alta disponibilidad, multi-cluster.

## Limites declarados (no son promesas)

- El sandbox comprueba que algo arranca, no que aguante trafico real.
- Comprueba que la red de kind aplica NetworkPolicy; si no, instala un CNI que lo haga.
- Muse Spark Open tiene 816B parametros; autoalojarlo no es realista; sirve por un proveedor. Confirma que socio lo sirve y si cuenta para el bonus de open weights.
- Los creditos de Meta son limitados: mide los tokens por ejecucion del benchmark.
- Revisa en las reglas oficiales si hay que declarar el uso de IA en el desarrollo; no escribas nada falso en el README.
- Las cifras del benchmark vienen del cluster de pruebas, no de uno de produccion.
- `metrics-server` para comparar con uso real no esta incluido en kind por defecto; si no se instala en 10 minutos, queda fuera.
