// Package reportserver exposes the generated report on localhost.
package reportserver

import (
	"context"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

//go:embed web/*
var webFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

// Server is a running report server.
type Server struct {
	URL string

	srv *http.Server
	ln  net.Listener
}

// Serve starts the report server. port 0 picks a free port.
func Serve(report *model.Report, host string, port int, open bool) (*Server, error) {
	if host == "" {
		host = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el puerto: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "reporte no disponible", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/assets/uplot.js", assetHandler("assets/uPlot.iife.min.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/assets/uplot.css", assetHandler("assets/uPlot.min.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/assets/app.css", webHandler("web/style.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/assets/app.js", webHandler("web/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("/api/report", reportHandler(report))
	mux.HandleFunc("/api/report.json", reportHandler(report))
	mux.HandleFunc("/api/timeline.csv", timelineHandler(report))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()

	displayHost := host
	if displayHost == "0.0.0.0" || displayHost == "::" || displayHost == "" {
		displayHost = "127.0.0.1"
	}
	_, actualPort, _ := net.SplitHostPort(ln.Addr().String())
	url := fmt.Sprintf("http://%s", net.JoinHostPort(displayHost, actualPort))
	if open {
		OpenBrowser(url)
	}
	return &Server{URL: url, srv: srv, ln: ln}, nil
}

// Close stops the HTTP server gracefully.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

func assetHandler(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := assetsFS.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(data)
	}
}

func webHandler(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := webFS.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(data)
	}
}

func reportHandler(report *model.Report) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	}
}

func timelineHandler(report *model.Report) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="loadforge-timeline.csv"`)
		writeTimelineCSV(w, report)
	}
}

func writeTimelineCSV(w io.Writer, report *model.Report) {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"second", "requests", "errors", "error_rate",
		"rate_rps", "target_rate_rps", "p50_ms", "p95_ms", "p99_ms",
		"client_cpu_percent", "client_cpu_share_percent", "client_rss_bytes", "client_goroutines",
	})
	for _, p := range report.Timeline {
		_ = cw.Write([]string{
			strconv.FormatFloat(p.T, 'f', 0, 64),
			strconv.FormatUint(p.Requests, 10),
			strconv.FormatUint(p.Errors, 10),
			trimFloat(p.ErrorRate, 4),
			trimFloat(p.Rate, 2),
			trimFloat(p.TargetRate, 2),
			trimFloat(p.P50, 2),
			trimFloat(p.P95, 2),
			trimFloat(p.P99, 2),
			trimFloat(p.ClientCPUPercent, 2),
			trimFloat(p.ClientCPUSharePercent, 2),
			strconv.FormatUint(p.ClientRSSBytes, 10),
			strconv.Itoa(p.ClientGoroutines),
		})
	}
	cw.Flush()
}

func trimFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// OpenBrowser opens url with the platform browser. Failures are silent: the
// URL is always printed by the caller.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "android":
		if p, err := exec.LookPath("termux-open-url"); err == nil {
			cmd = exec.Command(p, url)
		} else if p, err := exec.LookPath("am"); err == nil {
			cmd = exec.Command(p, "start", "-a", "android.intent.action.VIEW", "-d", url)
		} else {
			return
		}
	default:
		p, err := exec.LookPath("xdg-open")
		if err != nil {
			return
		}
		cmd = exec.Command(p, url)
	}
	_ = cmd.Start()
}
