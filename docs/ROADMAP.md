# Roadmap

Este documento organiza las mejoras de LoadForge en fases. Cada fase se
publica como una versión menor y debe quedar con tests, `go vet`, builds
cruzados y documentación actualizada antes de pasar a la siguiente.

## Fase 1 — Reporte y CI (`v0.3.0`)

- [x] `--export-html`: reporte autocontenido en un solo archivo.
- [x] `--fail-on-threshold`: código de salida distinto de 0 si el veredicto
  es `fail` o `limit`.
- [x] `--dry-run`: validar URL, auth, TLS y headers sin generar carga.
- [x] `loadforge serve <reporte.json>`: volver a abrir un reporte guardado.
- [x] Resumen de terminal adaptativo para anchos estrechos.
- [x] Muestrear CPU/memoria del cliente cada segundo y dibujarlas en el
  timeline del reporte web.

## Fase 2 — Exploración y comparación (`v0.4.0`)

- [x] `loadforge compare a.json b.json` y `--baseline`.
- [x] Etapas `step`, `spike`, `soak` y perfiles configurables.
- [x] Desglose de tiempos HTTP: DNS, TCP, TLS, TTFB y descarga.
- [x] Umbrales por código HTTP (por ejemplo, 404 = 0).
- [x] Exportar CSV completo además del timeline.

## Fase 3 — Realismo (`v0.5.0`)

- [x] Payloads desde CSV/JSON y templating (`{{.Iteration}}`, `{{.RandomID}}`).
- [x] OAuth2 client credentials, refresh y extracción de token con ruta de campos (`data.token`).
- [x] Modelo cerrado con VUs y think time.
- [x] Integración Termux: `termux-wake-lock`, batería, temperatura y red.
- [x] Pausa/reanudación y controles en la TUI.

## Fase 4A — Escenarios (`v0.6.0`)

- [x] Escenarios multi-paso en JSON con extracción de variables.
- [x] Métricas por paso en el reporte.
- [x] Multipart con `--form` (campos y archivos).

## Fase 4B — Protocolos y local (`v0.7.0`)

- [x] WebSocket (`--mode ws`, `--message`).
- [x] Escenarios YAML y pausas por paso (`pause_ms`).
- [x] Historial local con `--history-dir` y `loadforge history`.

## Fase 4C — gRPC y distribuido (`v0.7.1`)

- [ ] gRPC unario con reflexión/proto.
- [ ] Escenarios con pesos y variables complejas.
- [ ] Modo distribuido con agentes y coordinador.
- [x] Tendencias históricas en el reporte web.

## Fase 5 — Calidad y distribución (`v0.8.0`)

- [ ] CI con `-race`, cobertura, `golangci-lint`, `govulncheck` y e2e.
- [ ] Firma de binarios con cosign, SBOM y procedencia SLSA.
- [ ] Internacionalización (`--lang en`) y README en inglés.
- [ ] Dockerfile, `install.sh`, Homebrew, Scoop y paquete Termux.
- [ ] Completions, página man, CHANGELOG y plantillas de issues/PR.
