package engine

import (
	"fmt"
	"strings"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func buildVerdict(
	cfg Config,
	summary model.Summary,
	lat model.LatencyStats,
	timeline []model.TimelinePoint,
	statuses []model.StatusCount,
	errs []model.ErrorCount,
	interrupted, autoStopped bool,
	stopReason string,
) model.Verdict {
	v := model.Verdict{Status: model.VerdictPass}
	recs := recommendations(cfg, summary, lat, timeline, statuses, errs, autoStopped, stopReason)
	v.Recommendations = recs

	if interrupted {
		v.Status = model.VerdictInterrupted
		v.Title = "Prueba interrumpida"
		v.Message = fmt.Sprintf(
			"Se detuvo con %d peticiones completadas; los resultados son parciales y no representan la capacidad real.",
			summary.Requests,
		)
		return v
	}

	if summary.Requests == 0 {
		v.Status = model.VerdictFail
		v.Title = "Sin respuestas"
		v.Message = "No se completó ninguna petición. Revisa la URL, la red y las credenciales."
		return v
	}

	missing := countCodes(statuses, 404, 405)
	if float64(missing)/float64(summary.Requests) >= 0.5 {
		v.Status = model.VerdictFail
		v.Title = "Endpoint no encontrado"
		v.Message = fmt.Sprintf(
			"%d de %d peticiones recibieron 404/405. La URL o el método HTTP no coinciden con una ruta válida del backend. Revisa --url y -X.",
			missing, summary.Requests,
		)
		return v
	}

	authFailures := countCodes(statuses, 401) + countCodes(statuses, 403)
	if float64(authFailures)/float64(summary.Requests) >= 0.5 {
		v.Status = model.VerdictFail
		v.Title = "Autenticación rechazada"
		v.Message = fmt.Sprintf(
			"%d de %d peticiones recibieron 401/403. La API key o el esquema de autenticación no son válidos.",
			authFailures, summary.Requests,
		)
		return v
	}

	if summary.SuccessRate < 0.5 {
		v.Status = model.VerdictFail
		v.Title = "La mayoría de las peticiones fallaron"
		v.Message = fmt.Sprintf(
			"Solo el %.1f%% de las respuestas fue exitoso (2xx/3xx). Revisa los errores antes de medir capacidad.",
			summary.SuccessRate*100,
		)
		return v
	}

	breaking, at, found := findBreaking(timeline, cfg)
	latencyBreach := cfg.MaxP95 > 0 && lat.P95 > float64(cfg.MaxP95.Milliseconds())
	errorBreach := summary.ErrorRate > cfg.MaxErrorRate

	if cfg.Mode == ModeConstant {
		v.Status = model.VerdictPass
		v.Title = "Carga sostenida"
		v.Message = fmt.Sprintf(
			"El sistema sostuvo %.0f req/s con %.1f%% de éxito y P95 de %.0f ms.",
			cfg.Rate, summary.SuccessRate*100, lat.P95,
		)
		if latencyBreach || errorBreach {
			v.Status = model.VerdictFail
			v.Title = "Carga no sostenida"
			v.Message = fmt.Sprintf(
				"A %.0f req/s se superaron los umbrales: %.1f%% de errores y P95 de %.0f ms.",
				cfg.Rate, summary.ErrorRate*100, lat.P95,
			)
		}
		return v
	}

	// Ramp mode: a threshold breach is the capacity limit, not necessarily a
	// failed test. It is the result we were looking for.
	if found {
		v.Status = model.VerdictLimit
		v.Title = "Punto de quiebre detectado"
		v.BreakingRate = breaking
		v.BreakingAtSec = at
		limitText := fmt.Sprintf("errores > %.1f%%", cfg.MaxErrorRate*100)
		if cfg.MaxP95 > 0 {
			limitText = fmt.Sprintf("errores > %.1f%% o P95 > %.0f ms",
				cfg.MaxErrorRate*100, float64(cfg.MaxP95.Milliseconds()))
		}
		v.Message = fmt.Sprintf(
			"El sistema se mantuvo estable hasta ~%.0f req/s. A partir de ahí se superaron los umbrales (%s).",
			breaking, limitText,
		)
		if latencyBreach || errorBreach {
			v.Message += fmt.Sprintf(
				" En conjunto: %.1f%% de errores y P95 de %.0f ms.",
				summary.ErrorRate*100, lat.P95,
			)
		}
		if stopReason != "" {
			v.Message += " Detención automática: " + stopReason + "."
		}
		return v
	}

	if latencyBreach || errorBreach {
		v.Status = model.VerdictFail
		v.Title = "Umbrales superados"
		v.Message = fmt.Sprintf(
			"La prueba terminó con %.1f%% de errores y P95 de %.0f ms, por encima de los umbrales configurados.",
			summary.ErrorRate*100, lat.P95,
		)
		return v
	}

	v.Title = "Sin punto de quiebre"
	v.Message = fmt.Sprintf(
		"El sistema sostuvo toda la rampa (hasta %.0f req/s) con %.1f%% de éxito y P95 de %.0f ms.",
		cfg.MaxRate, summary.SuccessRate*100, lat.P95,
	)
	return v
}

func findBreaking(timeline []model.TimelinePoint, cfg Config) (rate float64, atSec float64, found bool) {
	lastHealthy := 0.0
	prevTarget := cfg.StartRate
	hasSample := false
	for _, p := range timeline {
		if p.Requests < 5 {
			prevTarget = p.TargetRate
			continue
		}
		hasSample = true
		healthy := p.ErrorRate <= cfg.MaxErrorRate
		if cfg.MaxP95 > 0 && p.P95 > float64(cfg.MaxP95.Milliseconds()) {
			healthy = false
		}
		if !healthy {
			if lastHealthy == 0 {
				lastHealthy = prevTarget
			}
			return lastHealthy, p.T, true
		}
		lastHealthy = p.TargetRate
		prevTarget = p.TargetRate
	}
	if hasSample {
		return lastHealthy, 0, false
	}
	return 0, 0, false
}

func countCodes(statuses []model.StatusCount, codes ...int) uint64 {
	var total uint64
	for _, s := range statuses {
		for _, c := range codes {
			if s.Code == c {
				total += s.Count
			}
		}
	}
	return total
}

func count5xx(statuses []model.StatusCount) uint64 {
	var total uint64
	for _, s := range statuses {
		if s.Code >= 500 && s.Code < 600 {
			total += s.Count
		}
	}
	return total
}

func recommendations(
	cfg Config,
	summary model.Summary,
	lat model.LatencyStats,
	timeline []model.TimelinePoint,
	statuses []model.StatusCount,
	errs []model.ErrorCount,
	autoStopped bool,
	stopReason string,
) []string {
	var out []string
	add := func(s string) {
		for _, existing := range out {
			if existing == s {
				return
			}
		}
		out = append(out, s)
	}

	if countCodes(statuses, 401, 403) > 0 {
		add("Recibiste 401/403: revisa --api-key-header y --api-key-prefix. Si usas Authorization, el prefijo suele ser \"Bearer \".")
	}
	if missing := countCodes(statuses, 404, 405); missing > 0 {
		add(fmt.Sprintf("Recibiste %d respuestas 404/405: revisa la ruta de la URL y el método HTTP; el endpoint puede no existir.", missing))
	}
	if countCodes(statuses, 429) > 0 {
		add("El backend aplicó rate limiting (429). El límite práctico está cerca del punto donde aparecieron esos códigos.")
	}
	if count5xx(statuses) > 0 {
		add("Hubo errores 5xx bajo carga: revisa logs del backend, pool de conexiones a la base de datos y timeouts internos.")
	}

	errText := strings.ToLower(errorText(errs))
	switch {
	case strings.Contains(errText, "timeout"), strings.Contains(errText, "deadline exceeded"):
		add("Se produjeron timeouts. Si el backend tarda legítimamente más, sube --timeout; si no, investiga el cuello de botella.")
	case strings.Contains(errText, "connection refused"):
		add("Conexiones rechazadas: comprueba que el servicio esté escuchando en esa URL y que el puerto sea correcto.")
	case strings.Contains(errText, "no such host"):
		add("No se pudo resolver el host: verifica la URL y el DNS del dispositivo.")
	case strings.Contains(errText, "certificate"), strings.Contains(errText, "tls"):
		add("Error TLS: usa --insecure solo contra entornos de prueba con certificados autofirmados.")
	}

	if cfg.MaxP95 > 0 && lat.P95 > float64(cfg.MaxP95.Milliseconds()) {
		add(fmt.Sprintf("El P95 llegó a %.0f ms (umbral: %d ms). Revisa endpoints lentos, consultas N+1 y añade caché donde tenga sentido.",
			lat.P95, cfg.MaxP95.Milliseconds()))
	}
	if lat.P95 > 0 && lat.P99 > lat.P95*3 {
		add("La latencia de cola es alta (P99 mucho mayor que P95): hay picos o bloqueos puntuales. Revisa GC, pools y locks.")
	}
	if summary.ErrorRate > cfg.MaxErrorRate {
		add(fmt.Sprintf("La tasa de error fue %.2f%% (umbral: %.2f%%). Prioriza estabilizar los errores antes de subir la carga.",
			summary.ErrorRate*100, cfg.MaxErrorRate*100))
	}

	if cfg.Mode == ModeRamp && len(timeline) > 0 {
		maxTarget := 0.0
		maxAchieved := 0.0
		for _, p := range timeline {
			if p.TargetRate > maxTarget {
				maxTarget = p.TargetRate
			}
			if p.Rate > maxAchieved {
				maxAchieved = p.Rate
			}
		}
		if maxTarget > 0 && maxAchieved < maxTarget*0.8 {
			add(fmt.Sprintf("No se alcanzó la tasa objetivo (máximo real %.0f req/s frente a %.0f req/s). Sube --max-workers o revisa la latencia del backend.",
				maxAchieved, maxTarget))
		}
	}

	if autoStopped && stopReason != "" {
		add("La prueba se detuvo automáticamente al superar los umbrales. Usa --no-autostop para exprimir el sistema hasta el final.")
	}
	if len(timeline) > 0 && cfg.Mode == ModeRamp {
		_, _, found := findBreaking(timeline, cfg)
		if !found && summary.Requests > 0 {
			add(fmt.Sprintf("No se encontró el límite en el rango probado. Repite con --max-rate mayor que %.0f req/s.", cfg.MaxRate))
		}
	}
	return out
}

func errorText(errs []model.ErrorCount) string {
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Message)
		b.WriteByte('\n')
	}
	return b.String()
}
