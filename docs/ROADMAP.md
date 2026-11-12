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

- [ ] `loadforge compare a.json b.json` y `--baseline`.
- [ ] Etapas `step`, `spike`, `soak` y perfiles configurables.
- [ ] Desglose de tiempos HTTP: DNS, TCP, TLS, TTFB y descarga.
- [ ] Umbrales por código HTTP (por ejemplo, 404 = 0) y por endpoint.
- [ ] Exportar CSV completo además del timeline.

## Fase 3 — Realismo (`v0.5.0`)

- [ ] Payloads desde CSV/JSON y templating (`{{.Iteration}}`, `{{.RandomID}}`).
- [ ] OAuth2 client credentials, refresh y extracción de token con JSONPath.
- [ ] Modelo cerrado con VUs y think time.
- [ ] Integración Termux: `termux-wake-lock`, batería, temperatura y red.
- [ ] Pausa/reanudación y controles en la TUI.

## Fase 4 — Protocolos y escala (`v0.6.0`)

- [ ] gRPC, WebSocket y multipart.
- [ ] Escenarios multi-paso en YAML/HCL con pesos y extracción de variables.
- [ ] Modo distribuido con agentes y coordinador.
- [ ] Historial local en SQLite y tendencias en el reporte web.

## Fase 5 — Calidad y distribución (`v0.7.0`)

- [ ] CI con `-race`, cobertura, `golangci-lint`, `govulncheck` y e2e.
- [ ] Firma de binarios con cosign, SBOM y procedencia SLSA.
- [ ] Internacionalización (`--lang en`) y README en inglés.
- [ ] Dockerfile, `install.sh`, Homebrew, Scoop y paquete Termux.
- [ ] Completions, página man, CHANGELOG y plantillas de issues/PR.
