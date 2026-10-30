# Umbra — Roadmap honesto

Diseño sin construir (no se presenta como implementado):

## Hoja de ruta

- **Helm**: chart `umbra` + subchart `umbra-crds`; el chart JAMÁS materializa secretos (solo `apiKeySecretRef`).
- **vCluster**: sandbox con mayor fidelidad creando clusters ligeros aislados dentro del host.
- **Imagen firmada + SBOM**: cosign + syft en el pipeline de build.
- **Exportación a SIEM** en JSON lines; informe de cumplimiento (CIS, Pod Security).
- **ValidatingAdmissionPolicy (CEL)**: prevención de clases de fallos arreglados (GA desde K8s 1.30).
- **Kill switch global**, presupuestos de cambios por hora/namespace, ventanas de congelación, excepciones por anotación con motivo y caducidad.
- **Enrutado de revisores** según la etiqueta de propietario del namespace.
- **Multi-tenant** con cuotas por equipo.
- **Integración con Vault** para Secrets.
- **Análisis de imágenes** con Trivy o Grype.
- **Modo CI**: `umbra scan ./manifests` con código de salida y GitHub Action.
- **Webhooks** genéricos para Slack/Teams; Jira para lo no arreglable.
- **Modo operador** (CRD RemediationPolicy como controlador): si no está listo para el Día 3, se entrega un demonio con configuración YAML y se dice con honestidad.
- **Ejecución de Muse Glimmer local**: sin medir todavía; serviría por un proveedor compatible con OpenAI.

## Límites declarados (no son promesas)

- El sandbox comprueba que algo arranca, no que aguante tráfico real. Dilo en el README.
- Comprueba que la red de kind aplica NetworkPolicy; si no, instala un CNI que lo haga.
- Muse Spark Open tiene 816B parámetros; autoalojarlo no es realista; sirve por un proveedor. Confirma qué socio lo sirve y si cuenta para el bonus de open weights.
- Los créditos de Meta son limitados: mide los tokens por ejecución del benchmark.
- Revisa en las reglas oficiales si hay que declarar el uso de IA en el desarrollo; no escribas nada falso en el README.
- Las cifras del benchmark vienen del cluster de pruebas, no de uno de producción.
- `metrics-server` para comparar con uso real no está incluido en kind por defecto; si no se instala en 10 minutos, queda fuera.
