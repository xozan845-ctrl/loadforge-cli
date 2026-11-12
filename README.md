# ⚡ LoadForge

[![CI](https://github.com/xozan845-ctrl/loadforge-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/xozan845-ctrl/loadforge-cli/actions/workflows/ci.yml)
[![Release](https://github.com/xozan845-ctrl/loadforge-cli/actions/workflows/release.yml/badge.svg)](https://github.com/xozan845-ctrl/loadforge-cli/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-22d3ee.svg)](LICENSE)

Pruebas de carga para APIs, desde la terminal. Le das una URL y una API key;
LoadForge genera carga real, mide latencias, errores y throughput, estima el
punto de quiebre y te sirve un reporte interactivo en `localhost`.

Funciona en **Termux (Android), Linux, macOS y Windows** con un único binario
estático. El reporte va embebido: no necesita internet ni dependencias externas.

```
  ⚡ LOADFORGE   ramp 1 → 50 req/s · 30s

  https://api.ejemplo.com/health

  ⠹ 12.4s / 30.0s   ████████████░░░░░░░░  41%

  Peticiones   3,241          Ritmo   82.0 / 82.0 req/s
  Latencia     P50 12 ms   P95 45 ms   P99 90 ms
  Salud        99.7% éxito   0.3% errores
  Umbrales     P95 ≤ 500 ms   errores ≤ 1.0%
```

Al terminar:

```
  ▲  PUNTO DE QUIEBRE DETECTADO
  El sistema se mantuvo estable hasta ~38 req/s...

  ➜  Reporte web listo
     http://127.0.0.1:41234
```

## Qué incluye

- **Motor de carga** basado en [Vegeta](https://github.com/tsenart/vegeta), con
  workers dinámicos y pacing exacto para rampas.
- **Dos modos**: `ramp` (sube de `start-rate` a `max-rate`) y `constant`
  (carga fija).
- **Métricas**: req/s reales vs. objetivo, P50/P90/P95/P99/P99.9, máximo,
  distribución de latencia, códigos HTTP, bytes y errores agrupados.
- **Timeline por segundo** con percentiles, para ver *cuándo* se degrada.
- **Veredicto automático**: apto, punto de quiebre, fallo o interrumpido.
- **Auto-stop** en rampas cuando el sistema se degrada de forma clara.
- **TUI en vivo** con barra de progreso, latencias y salud del backend.
- **Reporte web** oscuro, responsive y offline, con gráficas [uPlot](https://github.com/leeoniya/uPlot),
  descarga de JSON/CSV e impresión a PDF.
- **Entorno y coste del cliente**: CPU, memoria, carga media, goroutines, heap
  y GC del equipo que genera la carga, con aviso si el propio cliente se
  convierte en el cuello de botella. La CPU del cliente también se dibuja en
  el timeline junto a la carga real.
- **Reporte autocontenido**: `--export-html` genera un único archivo HTML con
  todo embebido, listo para compartir o guardar como artefacto de CI.
- **Modo CI**: `--fail-on-threshold` devuelve código de salida 1 cuando el
  veredicto no es `pass`, y `--dry-run` valida el objetivo sin generar carga.
- **Perfiles por etapas**: `step`, `spike` y `soak` con `--stage RATE:DURACION`,
  además de la rampa lineal y la carga constante.
- **Desglose de red**: DNS, conexión TCP, TLS, TTFB y descarga por separado.
- **Comparación con baseline**: `loadforge compare` o `--baseline` detectan
  regresiones de latencia, errores y throughput.
- **Umbrales por código HTTP**: por ejemplo `--max-status 404=0`.
- **CSV completo**: resumen por secciones además del timeline por segundo.
- **Payloads dinámicos**: `--data-file` con CSV/JSON y plantillas
  `{{.Iteration}}`, `{{.RandomID}}`, `{{.columna}}`.
- **OAuth2 client credentials**: token automático y renovación durante la prueba.
- **Modelo cerrado**: `--mode vu --vus N --think-time 1s`, con pausa/reanudación
  desde la TUI (`p`).
- **Termux**: wake-lock automático, batería, temperatura y tipo de red.
- **Escenarios multi-paso**: flujos en JSON con extracción de variables entre
  pasos y métricas por paso.
- **Multipart**: subida de formularios y archivos con `--form`.
- **WebSocket**: modo `ws` con VUs, mensajes y latencia de ida y vuelta.
- **Escenarios YAML** con pausas por paso.
- **Historial local**: `--history-dir` y comando `loadforge history`.
- **Redacción de secretos**: la API key nunca entra al reporte; los valores de
  query sensibles se reemplazan por `***`.

## Instalación

### Binarios ya compilados

En `dist/` hay binarios para:

| Plataforma | Archivo |
|---|---|
| Termux / Android ARM64 | `loadforge-termux-arm64` |
| Linux x86_64 | `loadforge-linux-amd64` |
| Linux ARM64 (Raspberry Pi, etc.) | `loadforge-linux-arm64` |
| Windows x86_64 | `loadforge-windows-amd64.exe` |
| macOS Apple Silicon | `loadforge-darwin-arm64` |
| macOS Intel | `loadforge-darwin-amd64` |

En **Termux**:

```bash
chmod +x loadforge-termux-arm64
./loadforge-termux-arm64 --version
```

Si quieres dejarlo en el PATH:

```bash
mkdir -p "$PREFIX/bin"
cp loadforge-termux-arm64 "$PREFIX/bin/loadforge"
chmod +x "$PREFIX/bin/loadforge"
loadforge --version
```

`$PREFIX/bin` ya está en el PATH de Termux. Si prefieres usar `~/.local/bin`,
añade esa carpeta al PATH en tu `~/.bashrc`:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

En **Linux** (x86_64 o ARM64):

```bash
chmod +x loadforge-linux-amd64
./loadforge-linux-amd64 --version
```

En **macOS Apple Silicon**:

```bash
chmod +x loadforge-darwin-arm64
./loadforge-darwin-arm64 --version
```

En **Windows** (PowerShell):

```powershell
.\loadforge-windows-amd64.exe --version
```

Opcionalmente, renómbralo a `loadforge.exe` para escribirlo más rápido.

### Desde el código

```bash
pkg install golang make  # Termux
go build -trimpath -o loadforge ./cmd/loadforge
```

Requiere Go 1.24 o superior. Para compilar todas las plataformas:

```bash
make dist
```

## Uso rápido

Los ejemplos usan `loadforge` asumiendo que el binario está en el `PATH`. Si
lo ejecutas directamente desde `dist/`, usa `./loadforge-termux-arm64` en
Termux, `./loadforge-linux-amd64` en Linux, `./loadforge-darwin-arm64` en
macOS o `.\loadforge-windows-amd64.exe` en Windows.

```bash
export LOADFORGE_API_KEY="tu-token"

loadforge run --url https://api.ejemplo.com/health
```

Por defecto hace una **rampa de 1 a 50 req/s durante 30 segundos**, con un
umbral de **1% de errores** y **P95 de 500 ms**. Si el backend se degrada de
forma clara, la prueba se detiene y reporta el punto de quiebre.

Antes de generar carga, LoadForge hace una **comprobación previa** con una
petición (solo en métodos seguros como `GET`/`HEAD`) para detectar rutas
inexistentes (`404/405`), credenciales inválidas (`401/403`) o un servicio
caído. Si encuentra uno de esos problemas, aborta con un mensaje claro en
lugar de malgastar la prueba. Desactívala con `--no-preflight`.

El token se envía como `Authorization: Bearer <token>`. Para otros esquemas:

```bash
# Header X-API-Key, sin prefijo
loadforge run --url https://api.ejemplo.com/v1/items \
  --api-key "$LOADFORGE_API_KEY" \
  --api-key-header X-API-Key \
  --api-key-prefix ""

# Header personalizado
loadforge run --url https://api.ejemplo.com/v1/items \
  -H "X-Tenant: acme" \
  -H "Accept: application/json"
```

## Ejemplos

**Carga constante y agresiva:**

```bash
LOADFORGE_API_KEY="$TOKEN" loadforge run \
  --url https://api.ejemplo.com/v1/search \
  --mode constant --rate 100 --duration 60s
```

**Encontrar el límite real:**

```bash
loadforge run \
  --url https://api.ejemplo.com/v1/items \
  --mode ramp --start-rate 10 --max-rate 500 --duration 2m \
  --max-error-rate 0.02 --max-p95 800ms
```

**Perfil por etapas (step / spike / soak):**

```bash
# Step: 10, 50 y 100 req/s, cada uno durante 30s
loadforge run --url https://api.ejemplo.com/health \
  --mode step --stage 10:30s --stage 50:30s --stage 100:30s

# Spike: subida y bajada brusca alrededor de 10 req/s
loadforge run --url https://api.ejemplo.com/health \
  --mode spike --stage 10:30s --stage 300:10s --stage 10:30s

# Soak: una hora a 20 req/s
loadforge run --url https://api.ejemplo.com/health \
  --mode soak --rate 20 --duration 1h
```

En `step` y `soak` cada etapa mantiene su tasa; en `spike` la tasa transiciona
linealmente desde la etapa anterior.

**Comparar con una ejecución anterior:**

```bash
loadforge compare reportes/antes.json reportes/despues.json

# O directamente al ejecutar
loadforge run --url https://api.ejemplo.com/health \
  --baseline reportes/antes.json

# Falla el pipeline si hay regresiones
loadforge compare antes.json despues.json --fail-on-regression
```

**Umbrales por código HTTP y CSV completo:**

```bash
loadforge run --url https://api.ejemplo.com/health \
  --max-status 404=0 --max-status 500=0 \
  --export-csv reportes/
```

Esto genera `reportes/summary.csv` y `reportes/timeline.csv`.

**Payloads dinámicos desde CSV:**

```bash
# data.csv: name,email
loadforge run --url https://api.ejemplo.com/v1/users \
  --method POST --data-file data.csv \
  --body '{"id":{{.Iteration}},"name":"{{.name}}","email":"{{.email}}","rid":"{{.RandomID}}"}'
```

Cada petición rota una fila del CSV/JSON. `{{.Iteration}}` empieza en 1 y
`{{.RandomID}}` genera un identificador distinto por petición.

**Modelo cerrado con usuarios virtuales:**

```bash
loadforge run --url https://api.ejemplo.com/health \
  --mode vu --vus 20 --think-time 1s --duration 5m
```

Aquí la carga la generan 20 usuarios que esperan 1s entre peticiones, en lugar
de un ritmo objetivo. Pulsa `p` en la TUI para pausar o reanudar.

**OAuth2 client credentials:**

```bash
loadforge run --url https://api.ejemplo.com/v1/items \
  --oauth2-token-url https://auth.ejemplo.com/oauth/token \
  --oauth2-client-id "$CLIENT_ID" \
  --oauth2-client-secret "$CLIENT_SECRET" \
  --oauth2-scope "read:items"
```

El token se obtiene antes de la prueba y se renueva automáticamente cuando
está a punto de caducar.

**Escenario multi-paso con extracción de token:**

```json
{
  "steps": [
    {
      "name": "login",
      "method": "POST",
      "path": "/auth/login",
      "body": "{\"user\":\"{{.user}}\"}",
      "extract": { "token": "access_token" }
    },
    {
      "name": "listar",
      "method": "GET",
      "path": "/v1/items",
      "headers": { "Authorization": "Bearer {{.token}}" }
    }
  ]
}
```

```bash
loadforge run --url https://api.ejemplo.com \
  --scenario flujo.json --mode vu --vus 10 --think-time 1s --duration 5m
```

Cada VU ejecuta los pasos en orden. Las variables extraídas de una respuesta
quedan disponibles en los pasos siguientes; también se pueden combinar con
`--data-file`. El reporte muestra métricas por paso.

**Multipart:**

```bash
loadforge run --url https://api.ejemplo.com/upload \
  --method POST --form name=demo --form avatar=@foto.png
```

**WebSocket:**

```bash
loadforge run --url wss://api.ejemplo.com/ws \
  --mode ws --vus 20 --message '{"type":"ping"}' \
  --think-time 100ms --duration 1m
```

Cada VU abre una conexión, envía el mensaje, espera la respuesta y mide el
round-trip. Asume un servidor de eco; los errores de conexión también cuentan.

**Historial local:**

```bash
loadforge run --url https://api.ejemplo.com/health \
  --history-dir ~/.local/share/loadforge/history

loadforge history
loadforge history --json
```

**POST con JSON:**

```bash
loadforge run \
  --url https://api.ejemplo.com/v1/items \
  --method POST \
  --body '{"name":"carga","price":10}' \
  --api-key "$TOKEN"
```

**Cuerpo desde archivo y headers extra:**

```bash
loadforge run \
  --url https://api.ejemplo.com/v1/orders \
  --method POST --body-file pedido.json \
  -H "X-Idempotency-Key: loadforge-1" \
  --api-key "$TOKEN"
```

**Solo JSON para CI, sin servidor:**

```bash
loadforge run \
  --url https://api.ejemplo.com/health \
  --json --save reportes/run-$(date +%Y%m%d-%H%M).json
```

**Reporte HTML autocontenido:**

```bash
loadforge run \
  --url https://api.ejemplo.com/health \
  --export-html reportes/run.html \
  --save reportes/run.json
```

El HTML incluye estilos, gráficas, datos y descargas: funciona sin conexión y
se puede abrir después con cualquier navegador.

**Como quality gate en CI:**

```bash
loadforge run \
  --url https://api.ejemplo.com/health \
  --mode constant --rate 100 --duration 30s \
  --json --no-serve \
  --export-html reporte.html \
  --fail-on-threshold
```

Si el veredicto es `fail` o `limit`, el proceso termina con código 1. Valida la
configuración sin lanzar carga con:

```bash
loadforge run --url https://api.ejemplo.com/health --dry-run
```

**Reabrir un reporte guardado:**

```bash
loadforge serve reportes/run.json
```

### Servidor falso local

El repo incluye `cmd/mockserver`, un backend de prueba que simula latencia y
errores. No se distribuye como binario: compílalo desde el código.

```bash
cd ~/loadforge
go build -o dist/mockserver ./cmd/mockserver

# Terminal 1: arranca el servidor en 127.0.0.1:9099
./dist/mockserver
```

| Endpoint | Comportamiento |
|---|---|
| `/healthz` | Respuesta inmediata `{"ok":true}` |
| `/api/data` | Latencia simulada y ~1% de errores 500 |
| `/secure` | Exige `Authorization: Bearer demo` o `X-API-Key: demo` |

Ajusta la simulación con flags:

```bash
# 5% de errores y más latencia para ver un veredicto negativo
./dist/mockserver -addr 127.0.0.1:9099 -latency 20ms -jitter 30ms -fail 0.05
```

En la **Terminal 2** ejecuta las pruebas:

```bash
# Rampa de carga
loadforge run --url http://127.0.0.1:9099/api/data \
  --mode ramp --start-rate 1 --max-rate 50 --duration 20s

# Carga constante
loadforge run --url http://127.0.0.1:9099/api/data \
  --mode constant --rate 30 --duration 10s

# Probar autenticación
loadforge run --url http://127.0.0.1:9099/secure \
  --api-key demo --mode constant --rate 5 --duration 5s

# Solo JSON, sin TUI ni servidor de reporte
loadforge run --url http://127.0.0.1:9099/api/data \
  --mode constant --rate 20 --duration 5s --json > reporte.json

# Reporte en un puerto fijo y sin abrir el navegador
loadforge run --url http://127.0.0.1:9099/api/data \
  --mode constant --rate 20 --duration 10s --no-open --port 8080
# Luego abre http://127.0.0.1:8080
```

Detén el servidor falso con `Ctrl+C` en su terminal, o con `kill %1` si lo
dejaste en segundo plano.

Si prefieres no compilar nada, también sirve cualquier servidor local:

```bash
python3 -m http.server 9099 --bind 127.0.0.1
loadforge run --url http://127.0.0.1:9099/ --mode constant --rate 10 --duration 5s
```

## Flags principales

| Flag | Default | Descripción |
|---|---|---|
| `-u, --url` | — | URL completa (`http://` o `https://`). Obligatorio. |
| `-k, --api-key` | env `LOADFORGE_API_KEY` | API key. Mejor por variable de entorno. |
| `--api-key-header` | `Authorization` | Header donde va la key. |
| `--api-key-prefix` | `Bearer ` | Prefijo del valor. Vacío para `X-API-Key`. |
| `-X, --method` | `GET` | Método HTTP. |
| `-H, --header` | — | Header extra `Clave: Valor`, repetible. |
| `--body` / `--body-file` | — | Cuerpo de la petición (admite placeholders). |
| `--data-file` | — | CSV o JSON con filas para templating. |
| `--scenario` | — | Flujo multi-paso en JSON (requiere `--mode vu`). |
| `--form` | — | Campo multipart `name=value` o `name=@archivo`. |
| `--oauth2-token-url` | — | Endpoint OAuth2 client_credentials. |
| `--oauth2-client-id` / `--oauth2-client-secret` | — | Credenciales OAuth2. |
| `--oauth2-scope` | — | Scope solicitado. |
| `--oauth2-token-field` | `access_token` | Campo del token (admite `data.token`). |
| `--oauth2-auth-style` | `body` | Credenciales en `body` o `basic`. |
| `--content-type` | `application/json` si hay body | Content-Type explícito. |
| `--mode` | `ramp` | `ramp`, `constant`, `step`, `spike` o `soak`. |
| `-r, --rate` | `20` | req/s en modo `constant` o `soak`. |
| `--start-rate` | `1` | req/s inicial de la rampa. |
| `--max-rate` | `50` | req/s máximo de la rampa. |
| `--stage` | — | Etapa `RATE:DURACION`, repetible para `step`/`spike`/`soak`. |
| `-d, --duration` | `30s` | Duración (`30s`, `2m`, `1h`). |
| `-c, --workers` | `10` | Workers iniciales (modelo abierto). |
| `--max-workers` | auto | Tope de concurrencia (auto = 2× `max-rate`, mín. 100). |
| `--vus` | `10` | Usuarios virtuales en `--mode vu` o conexiones en `ws`. |
| `--think-time` | `0` | Pausa por VU entre peticiones (ej. `1s`). |
| `--message` | `ping` | Mensaje de texto en `--mode ws`. |
| `-t, --timeout` | `10s` | Timeout por petición. |
| `--insecure` | `false` | No verificar TLS (solo pruebas). |
| `--max-body` | `4096` | Bytes de respuesta leídos por petición. |
| `--max-error-rate` | `0.01` | SLO de errores (0.01 = 1%). |
| `--max-p95` | `500ms` | SLO de latencia P95 (`0` lo desactiva). |
| `--max-status` | — | Máximo de respuestas por código, `CODE=COUNT` (repetible). |
| `--no-autostop` | `false` | No cortar la rampa al degradarse. |
| `--no-preflight` | `false` | No comprobar la URL antes de generar carga. |
| `--no-wake-lock` | `false` | No usar `termux-wake-lock` durante la prueba. |
| `--host` | `127.0.0.1` | Interfaz del reporte. |
| `--port` | `0` | Puerto (`0` = aleatorio). |
| `--no-open` | `false` | No abrir el navegador. |
| `--no-serve` | `false` | No levantar el servidor. |
| `--save` | — | Guardar el JSON en una ruta o directorio. |
| `--history-dir` | — | Guardar una copia en el historial local. |
| `--baseline` | — | Comparar la ejecución contra un reporte guardado. |
| `--export-html` | — | Guardar un reporte HTML autocontenido. |
| `--export-csv` | — | Guardar resumen y timeline en CSV. |
| `--json` | `false` | Imprimir JSON y salir (sin TUI ni servidor). |
| `--quiet` | `false` | Sin progreso en vivo. |
| `--dry-run` | `false` | Validar URL, auth y TLS sin generar carga. |
| `--fail-on-threshold` | `false` | Salir con código 1 si el veredicto no es `pass`. |

## Cómo leer el reporte

### Veredicto

- **✔ Sin punto de quiebre / Carga sostenida**: no se superaron los umbrales.
- **▲ Punto de quiebre detectado**: en una rampa, se encontró la carga donde
  el P95 o la tasa de error cruzaron el SLO. El número grande es la última
  tasa objetivo que se mantuvo saludable.
- **✕ Fallo**: autenticación rechazada, la mayoría de peticiones falló o los
  umbrales se superaron en una carga constante.
- **✕ Endpoint no encontrado**: la mayoría de respuestas fueron `404/405`;
  revisa la ruta de `--url` y el método `-X`.
- **◼ Interrumpido**: pulsaste Ctrl+C; los datos son parciales.

### Percentiles

El **P95 = 500 ms** significa que el 95% de las respuestas tardó 500 ms o
menos. Los promedios esconden colas: mira siempre P95/P99, no solo la media.
La distribución de latencia muestra si hay una cola larga.

### Punto de quiebre

Se evalúa cada segundo completado. El punto de quiebre es la última tasa
objetivo con `errores ≤ max-error-rate` y `P95 ≤ max-p95`. El auto-stop usa un
margen de **2×** sobre el SLO de errores y **1.5×** sobre el de latencia, y
exige al menos 20 peticiones en una ventana de 5 segundos, para no abortar por
un par de fallos aislados. El reporte final sí usa los umbrales estrictos.

### Timeline

Cada punto agrupa las respuestas **completadas** en ese segundo. Si el backend
tarda más que la duración, esas respuestas en vuelo aparecen al final; el
`--timeout` acota la espera.

El reporte web añade una gráfica de **presión del cliente**: CPU del generador
frente a la carga real. Si la CPU sube mientras los req/s reales se estancan,
el límite está en el equipo que genera la carga. El CSV incluye esas columnas
(`client_cpu_percent`, `client_rss_bytes`, etc.).

### Desglose de tiempos

La latencia total se divide en **DNS**, **conexión TCP**, **TLS**, **TTFB**
(tiempo hasta el primer byte) y **descarga**. Las conexiones reutilizadas no
pagan DNS/TCP/TLS, por eso el porcentaje de reutilización ayuda a interpretar
esos percentiles. Si el TTFB domina, el cuello está en el backend; si domina
la descarga, en el tamaño de la respuesta o la red.

### Comparación con baseline

`loadforge compare base.json actual.json` (o `--baseline` al ejecutar) compara
P95, P99, latencia media, errores, éxito, throughput y ritmo. Marca regresión
cuando la latencia sube más de **20%**, el throughput baja más de **10%** o la
tasa de error sube más de **1 punto porcentual**. Usa `--fail-on-regression`
para convertirlo en quality gate.

Compara siempre ejecuciones con el **mismo objetivo y perfil**: si cambian la
URL, el modo o las etapas, las cifras no son directamente comparables.

### Entorno y rendimiento del cliente

Una prueba de carga mide dos cosas: el backend y la máquina que genera la
carga. El resumen final y el reporte web incluyen:

- **Entorno**: sistema operativo, arquitectura, kernel, CPUs lógicas,
  `GOMAXPROCS`, memoria total y disponible, carga media (1/5/15 min) y versión
  de Go.
- **Cliente**: CPU consumida por LoadForge (tiempo de usuario y sistema, % de
  un núcleo y % del equipo), RSS máximo, goroutines máximas y finales, heap en
  uso, memoria asignada total y actividad del recolector de basura.

Estos datos son best-effort y dependen de la plataforma. En Termux/Android y
Linux se leen de `/proc`; en macOS y Windows se usan las APIs del sistema. Si
algún valor no está disponible, se omite.

Si el cliente usa **≥ 70% de su capacidad de CPU** y la tasa real queda por
debajo del 90% de la objetivo, el reporte muestra un aviso de **posible cuello
de botella en el cliente**. En ese caso, parte de la degradación puede ser del
equipo que genera la carga y no del backend: reduce `--max-rate`, usa menos
workers o prueba desde otra máquina antes de culpar al servidor.

## Seguridad

- Prefiere `LOADFORGE_API_KEY` a `--api-key`: los argumentos quedan en el
  historial y en `ps`.
- La API key nunca se escribe en el reporte. Solo se listan nombres de headers.
- Credenciales y parámetros sensibles de la URL (`token`, `secret`, `apikey`,
  `password`, `signature`, etc.) se redactan como `***`.
- El servidor escucha en `127.0.0.1` por defecto y **no tiene autenticación**.
  Si usas `--host 0.0.0.0`, cualquiera en la red podrá abrir el reporte.
- `--insecure` desactiva la verificación TLS. Úsalo solo contra entornos de
  prueba.
- **No pruebes sistemas que no te pertenezcan** o para los que no tengas
  autorización. Una prueba de carga es, literalmente, un ataque de denegación
  de servicio voluntario.

## Limitaciones

- Genera carga desde **una sola máquina**. En Termux, el cuello de botella
  puede ser el propio teléfono (CPU, red móvil, ahorro de energía). Compara
  siempre `objetivo` vs. `real` y revisa el bloque de **rendimiento del
  cliente** antes de atribuir la caída al backend.
- Android puede matar el proceso en segundo plano. Mantén Termux abierto y,
  si hace falta, usa `termux-wake-lock`.
- Solo HTTP/HTTPS. No hay WebSocket, gRPC ni cargas multipart en esta versión.
- Lee hasta `--max-body` bytes por respuesta (4 KB por defecto) para mantener
  el consumo de memoria bajo; la latencia incluye esa lectura.
- No hay workers distribuidos ni almacenamiento histórico; guarda el JSON con
  `--save`.

## Desarrollo

La arquitectura, los patrones aplicados, los compromisos y la ruta hacia una
Clean/Hexagonal estricta están documentados en
[**docs/ARCHITECTURE.md**](docs/ARCHITECTURE.md). Las próximas mejoras, por
fases, están en [**docs/ROADMAP.md**](docs/ROADMAP.md).

```bash
go test ./...                # unitarios + integración con httptest
go run ./cmd/mockserver      # backend de prueba en :9099
go run ./cmd/loadforge run --url http://127.0.0.1:9099/api/data
```

Estructura:

```
cmd/loadforge        entrada del CLI
cmd/mockserver       backend de prueba (no se distribuye)
internal/engine      Vegeta, pacer, agregación, veredicto
internal/model       modelo JSON del reporte
internal/tui         dashboard Bubble Tea
internal/reportserver servidor y SPA embebida
internal/cli         Cobra, flags y resumen final
```

## Atribución

LoadForge se apoya en proyectos open source: Vegeta, Bubble Tea, Lip Gloss,
Cobra, HdrHistogram-go y uPlot. Ver [THIRD_PARTY.md](THIRD_PARTY.md).

## Licencia

MIT. Ver [LICENSE](LICENSE).
