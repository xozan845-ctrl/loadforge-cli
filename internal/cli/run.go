package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xozan845-ctrl/loadforge-cli/internal/engine"
	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
	"github.com/xozan845-ctrl/loadforge-cli/internal/reportserver"
	"github.com/xozan845-ctrl/loadforge-cli/internal/tui"
	"github.com/xozan845-ctrl/loadforge-cli/internal/version"
)

type runOptions struct {
	url          string
	apiKey       string
	apiKeyHeader string
	apiKeyPrefix string
	method       string
	headers      []string
	body         string
	bodyFile     string
	contentType  string

	mode      string
	rate      float64
	startRate float64
	maxRate   float64
	duration  time.Duration

	workers    int
	maxWorkers int
	timeout    time.Duration
	insecure   bool
	maxBody    int64

	maxErrorRate float64
	maxP95       time.Duration
	noAutoStop   bool

	host    string
	port    int
	noOpen  bool
	noServe bool
	save    string
	jsonOut bool
	quiet   bool
}

func newRunCmd() *cobra.Command {
	opts := &runOptions{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Ejecuta una prueba de carga y abre el reporte",
		Long: `Ejecuta la prueba, muestra métricas en vivo y levanta un servidor local
con el reporte completo.

Por defecto hace una rampa de 1 a 50 req/s durante 30 segundos. En modo ramp
se detiene automáticamente si el sistema se degrada de forma clara: 2× el SLO
de errores o 1.5× el SLO de latencia, con al menos 20 peticiones en una
ventana de 5 segundos.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := buildConfig(opts)
			if err != nil {
				return err
			}
			return executeRun(cfg, opts)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&opts.url, "url", "u", "", "URL del endpoint a probar (obligatorio)")
	f.StringVarP(&opts.apiKey, "api-key", "k", "", "API key (mejor usa LOADFORGE_API_KEY)")
	f.StringVar(&opts.apiKeyHeader, "api-key-header", "Authorization", "Header donde enviar la API key")
	f.StringVar(&opts.apiKeyPrefix, "api-key-prefix", "Bearer ", "Prefijo del valor de la API key (vacío para X-API-Key)")
	f.StringVarP(&opts.method, "method", "X", "GET", "Método HTTP")
	f.StringArrayVarP(&opts.headers, "header", "H", nil, "Header extra 'Clave: Valor' (repetible)")
	f.StringVar(&opts.body, "body", "", "Cuerpo de la petición")
	f.StringVar(&opts.bodyFile, "body-file", "", "Archivo con el cuerpo de la petición")
	f.StringVar(&opts.contentType, "content-type", "", "Content-Type (por defecto application/json si hay body)")

	f.StringVar(&opts.mode, "mode", engine.ModeRamp, "Modo: ramp (rampa) o constant (carga fija)")
	f.Float64VarP(&opts.rate, "rate", "r", 20, "Peticiones por segundo en modo constant")
	f.Float64Var(&opts.startRate, "start-rate", 1, "req/s inicial en modo ramp")
	f.Float64Var(&opts.maxRate, "max-rate", 50, "req/s máximo en modo ramp")
	f.DurationVarP(&opts.duration, "duration", "d", 30*time.Second, "Duración de la prueba (ej. 30s, 2m)")

	f.IntVarP(&opts.workers, "workers", "c", 10, "Workers iniciales")
	f.IntVar(&opts.maxWorkers, "max-workers", 0, "Máximo de workers (0 = automático)")
	f.DurationVarP(&opts.timeout, "timeout", "t", 10*time.Second, "Timeout por petición")
	f.BoolVar(&opts.insecure, "insecure", false, "No verificar certificados TLS (solo pruebas)")
	f.Int64Var(&opts.maxBody, "max-body", 4096, "Bytes máximos de respuesta a leer por petición")

	f.Float64Var(&opts.maxErrorRate, "max-error-rate", 0.01, "Tasa de error máxima, 0.01 = 1%")
	f.DurationVar(&opts.maxP95, "max-p95", 500*time.Millisecond, "P95 máximo aceptable (0 para desactivar)")
	f.BoolVar(&opts.noAutoStop, "no-autostop", false, "No detener la rampa al superar los umbrales")

	f.StringVar(&opts.host, "host", "127.0.0.1", "Host donde servir el reporte")
	f.IntVar(&opts.port, "port", 0, "Puerto del reporte (0 = aleatorio)")
	f.BoolVar(&opts.noOpen, "no-open", false, "No abrir el navegador automáticamente")
	f.BoolVar(&opts.noServe, "no-serve", false, "No levantar el servidor del reporte")
	f.StringVar(&opts.save, "save", "", "Guardar el reporte JSON en esta ruta")
	f.BoolVar(&opts.jsonOut, "json", false, "Imprimir el reporte JSON y salir (sin TUI ni servidor)")
	f.BoolVar(&opts.quiet, "quiet", false, "Sin barra de progreso en vivo")

	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func buildConfig(opts *runOptions) (engine.Config, error) {
	headers := http.Header{}
	for _, raw := range opts.headers {
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			return engine.Config{}, fmt.Errorf("header inválido %q: usa 'Clave: Valor'", raw)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return engine.Config{}, fmt.Errorf("header inválido %q: falta el nombre", raw)
		}
		headers.Add(key, value)
	}

	apiKey := opts.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("LOADFORGE_API_KEY")
	}
	if apiKey != "" && headers.Get(opts.apiKeyHeader) == "" {
		headers.Set(opts.apiKeyHeader, opts.apiKeyPrefix+apiKey)
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", version.UserAgent())
	}

	var body []byte
	switch {
	case opts.body != "" && opts.bodyFile != "":
		return engine.Config{}, fmt.Errorf("usa --body o --body-file, no ambos")
	case opts.bodyFile != "":
		data, err := os.ReadFile(opts.bodyFile)
		if err != nil {
			return engine.Config{}, fmt.Errorf("no se pudo leer --body-file: %w", err)
		}
		body = data
	case opts.body != "":
		body = []byte(opts.body)
	}
	if opts.contentType != "" {
		headers.Set("Content-Type", opts.contentType)
	} else if len(body) > 0 && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}

	mode := strings.ToLower(opts.mode)
	startRate, maxRate := opts.startRate, opts.maxRate
	if mode == engine.ModeConstant {
		startRate, maxRate = opts.rate, opts.rate
	}

	maxWorkers := opts.maxWorkers
	if maxWorkers <= 0 {
		auto := int(math.Ceil(maxRate * 2))
		if auto < 100 {
			auto = 100
		}
		if auto > 2000 {
			auto = 2000
		}
		maxWorkers = auto
	}
	if maxWorkers < opts.workers {
		maxWorkers = opts.workers
	}

	cfg := engine.Config{
		URL:          strings.TrimSpace(opts.url),
		Method:       strings.ToUpper(opts.method),
		Headers:      headers,
		Body:         body,
		Mode:         mode,
		Rate:         opts.rate,
		StartRate:    startRate,
		MaxRate:      maxRate,
		Duration:     opts.duration,
		Workers:      opts.workers,
		MaxWorkers:   maxWorkers,
		Timeout:      opts.timeout,
		Insecure:     opts.insecure,
		MaxBody:      opts.maxBody,
		MaxErrorRate: opts.maxErrorRate,
		MaxP95:       opts.maxP95,
		AutoStop:     !opts.noAutoStop && mode == engine.ModeRamp,
		Version:      version.Version,
	}
	if err := cfg.Validate(); err != nil {
		return engine.Config{}, err
	}
	return cfg, nil
}

func executeRun(cfg engine.Config, opts *runOptions) error {
	var (
		report *model.Report
		err    error
	)
	if opts.jsonOut || opts.quiet || !term.IsTerminal(int(os.Stdout.Fd())) {
		report, err = runPlain(cfg, opts.quiet)
	} else {
		report, err = tui.Run(cfg)
	}
	if err != nil {
		return err
	}

	var savedPath string
	if opts.save != "" {
		savedPath, err = saveReport(opts.save, report)
		if err != nil {
			fmt.Fprintln(os.Stderr, warnStyle.Render("⚠ ")+"no se pudo guardar el reporte: "+err.Error())
			savedPath = ""
		}
	}

	if opts.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printSummary(report, savedPath)

	if opts.noServe {
		if savedPath == "" {
			fmt.Println("  " + mutedStyle.Render("Servidor omitido (--no-serve). Usa --save para exportar el JSON."))
			fmt.Println()
		}
		return nil
	}

	if opts.host != "127.0.0.1" && opts.host != "localhost" && opts.host != "::1" {
		fmt.Fprintln(os.Stderr, warnStyle.Render("⚠ ")+"el reporte se expondrá fuera de localhost; cualquiera en la red podrá verlo.")
	}
	srv, err := reportserver.Serve(report, opts.host, opts.port, !opts.noOpen)
	if err != nil {
		return err
	}
	defer srv.Close()

	printReportURL(srv.URL)
	waitForInterrupt()
	fmt.Println()
	fmt.Println("  " + mutedStyle.Render("Servidor cerrado."))
	return nil
}

func runPlain(cfg engine.Config, quiet bool) (*model.Report, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	showProgress := !quiet && term.IsTerminal(int(os.Stderr.Fd()))
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	var last time.Time
	frame := 0

	report, err := engine.Run(ctx, cfg, func(p model.Progress) {
		if !showProgress || time.Since(last) < time.Second {
			return
		}
		last = time.Now()
		fmt.Fprintf(os.Stderr, "\r\033[2K  %s %.1fs / %.1fs · %d req · %.1f req/s · P95 %s · %.2f%% err",
			frames[frame%len(frames)], p.Elapsed.Seconds(), p.Total.Seconds(),
			p.Requests, p.Rate, formatDuration(p.P95), p.ErrorRate*100)
		frame++
	})
	if showProgress {
		fmt.Fprint(os.Stderr, "\r\033[2K")
	}
	return report, err
}

func saveReport(path string, report *model.Report) (string, error) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		path = filepath.Join(path, "loadforge-"+time.Now().Format("20060102-150405")+".json")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func waitForInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
}
