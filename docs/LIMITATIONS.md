# LIMITATIONS.md — lo que Umbra no hace (ni finge hacer)

- **Sin Helm ni tráfico real en el sandbox.** El sandbox comprueba que algo
  arranca y pasa sondas, no que aguante carga. Cae del catálogo lo que exija
  reproducir tráfico (F04 se verifica por límites + reinicios estables, no por
  carga real).
- **Sin producción real.** El manager/producción solo propone; aplicar es un
  acto humano (`umbra approve` / anotación). Nunca hay auto-apply.
- **Sin Prometheus.** Comparar requests contra consumo histórico exige series
  temporales; `metrics-server` es una instantánea y no decide nada. La
  optimización de costes va al roadmap.
- **Calendario CronJob parcial.** `NextCronRuns` solo expande `*`, `*/k` y
  números en minuto/hora; el resto es "desconocido", nunca sano.
- **Detección estática, no runtime.** Solapes reales de CronJob, QoS bajo
  presión o colisiones de horario multi-CronJob necesitan observación en vivo;
  el detector marca el riesgo, el evento lo confirma.
- **Roles sin uso.** Requiere audit log real con historial; fuera de alcance.
- **Modelo local.** Glimmer vía endpoint OpenAI-compatible: cableado
  (`MODEL_BASE_URL`), sin medir.
- **Red de salida.** El cluster solo necesita llegar a la API del modelo;
  hoy el agente corre en el host, así que esto no afecta a las imágenes.
  Si el agente se mueve al manager: `FROM scratch` no trae certificados
  raíz y la llamada HTTPS fallaría — habría que copiar el paquete CA y
  abrir la salida en su NetworkPolicy (ver comentario en
  `Dockerfile.manager`).
