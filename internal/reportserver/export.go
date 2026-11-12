package reportserver

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// ExportHTML writes a single self-contained HTML file: markup, styles, uPlot,
// app code and the report data are all inlined, so the file works offline and
// can be shared as a CI artifact or attached to a ticket.
func ExportHTML(report *model.Report, path string) (string, error) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		path = filepath.Join(path, "loadforge-"+time.Now().Format("20060102-150405")+".html")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	index, err := webFS.ReadFile("web/index.html")
	if err != nil {
		return "", err
	}
	appCSS, err := webFS.ReadFile("web/style.css")
	if err != nil {
		return "", err
	}
	appJS, err := webFS.ReadFile("web/app.js")
	if err != nil {
		return "", err
	}
	plotCSS, err := assetsFS.ReadFile("assets/uPlot.min.css")
	if err != nil {
		return "", err
	}
	plotJS, err := assetsFS.ReadFile("assets/uPlot.iife.min.js")
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(report)
	if err != nil {
		return "", err
	}

	html := string(index)
	replacements := []struct {
		old string
		new string
	}{
		{
			`<link rel="stylesheet" href="/assets/uplot.css">`,
			"<style>\n" + string(plotCSS) + "\n</style>",
		},
		{
			`<link rel="stylesheet" href="/assets/app.css">`,
			"<style>\n" + string(appCSS) + "\n</style>",
		},
		{
			`<script src="/assets/uplot.js"></script>`,
			"<script>\n" + string(plotJS) + "\n</script>",
		},
		{
			`<script src="/assets/app.js"></script>`,
			"<script>window.__LOADFORGE_REPORT__=" + string(data) + ";</script>\n" +
				"<script>\n" + string(appJS) + "\n</script>",
		},
	}
	for _, r := range replacements {
		if !strings.Contains(html, r.old) {
			return "", fmt.Errorf("plantilla HTML inesperada: no se encontró %q", r.old)
		}
		html = strings.Replace(html, r.old, r.new, 1)
	}

	if err := os.WriteFile(path, []byte(html), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// ExportCSV writes two files: a flat summary (section,metric,value) and the
// per-second timeline. If path is a directory, the files are summary.csv and
// timeline.csv; otherwise path is the summary and the timeline gets a
// "-timeline" suffix.
func ExportCSV(report *model.Report, path string) (summaryPath, timelinePath string, err error) {
	asDir := strings.HasSuffix(path, string(os.PathSeparator))
	if fi, statErr := os.Stat(path); statErr == nil {
		asDir = fi.IsDir()
	} else if filepath.Ext(path) == "" {
		// A path without extension is treated as a directory to create.
		asDir = true
	}

	if asDir {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return "", "", err
		}
		summaryPath = filepath.Join(path, "summary.csv")
		timelinePath = filepath.Join(path, "timeline.csv")
	} else {
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)
		summaryPath = path
		timelinePath = base + "-timeline.csv"
	}
	for _, p := range []string{summaryPath, timelinePath} {
		if dir := filepath.Dir(p); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", "", err
			}
		}
	}

	if err := writeSummaryCSV(report, summaryPath); err != nil {
		return "", "", err
	}
	f, err := os.Create(timelinePath)
	if err != nil {
		return "", "", err
	}
	writeTimelineCSV(f, report)
	if err := f.Close(); err != nil {
		return "", "", err
	}
	return summaryPath, timelinePath, nil
}

func writeSummaryCSV(report *model.Report, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	cw := csv.NewWriter(f)
	row := func(section, metric, value string) {
		_ = cw.Write([]string{section, metric, value})
	}
	_ = cw.Write([]string{"section", "metric", "value"})

	m, s, l, t, tm, e, v := report.Meta, report.Summary, report.Latency, report.Thresholds, report.Timings, report.Environment, report.Verdict
	row("meta", "tool", m.Tool)
	row("meta", "version", m.Version)
	row("meta", "target", m.Target)
	row("meta", "method", m.Method)
	row("meta", "mode", m.Mode)
	row("meta", "started_at", m.StartedAt.Format(time.RFC3339))
	row("meta", "finished_at", m.FinishedAt.Format(time.RFC3339))
	row("meta", "duration_ms", strconv.FormatInt(m.Duration, 10))
	row("meta", "planned_duration_ms", strconv.FormatInt(m.PlannedDuration, 10))
	row("meta", "workers", strconv.Itoa(m.Workers))
	row("meta", "max_workers", strconv.Itoa(m.MaxWorkers))
	for i, stage := range m.Stages {
		row("meta", fmt.Sprintf("stage_%d_rate", i+1), trimFloat(stage.Rate, 2))
		row("meta", fmt.Sprintf("stage_%d_duration_ms", i+1), strconv.FormatInt(stage.DurationMs, 10))
	}

	row("summary", "requests", strconv.FormatUint(s.Requests, 10))
	row("summary", "rate_rps", trimFloat(s.Rate, 2))
	row("summary", "throughput_rps", trimFloat(s.Throughput, 2))
	row("summary", "success_rate", trimFloat(s.SuccessRate, 4))
	row("summary", "error_rate", trimFloat(s.ErrorRate, 4))
	row("summary", "bytes_in", strconv.FormatUint(s.BytesIn, 10))
	row("summary", "bytes_out", strconv.FormatUint(s.BytesOut, 10))

	for _, p := range []struct {
		name string
		val  float64
	}{
		{"min_ms", l.Min}, {"mean_ms", l.Mean}, {"p50_ms", l.P50},
		{"p90_ms", l.P90}, {"p95_ms", l.P95}, {"p99_ms", l.P99},
		{"p999_ms", l.P999}, {"max_ms", l.Max},
	} {
		row("latency", p.name, trimFloat(p.val, 3))
	}

	for _, code := range report.StatusCodes {
		row("status", strconv.Itoa(code.Code), strconv.FormatUint(code.Count, 10))
	}
	for _, ec := range report.Errors {
		row("error", ec.Message, strconv.FormatUint(ec.Count, 10))
	}

	row("thresholds", "max_error_rate", trimFloat(t.MaxErrorRate, 4))
	row("thresholds", "max_p95_ms", trimFloat(t.MaxP95Ms, 2))
	for _, limit := range t.MaxStatus {
		row("thresholds", fmt.Sprintf("max_status_%d", limit.Code), strconv.FormatUint(limit.Max, 10))
	}

	row("timings", "samples", strconv.FormatUint(tm.Samples, 10))
	row("timings", "reused_percent", trimFloat(tm.ReusedPercent, 2))
	for _, p := range []struct {
		name string
		val  model.PhaseStats
	}{
		{"dns", tm.DNS}, {"connect", tm.Connect}, {"tls", tm.TLS},
		{"ttfb", tm.TTFB}, {"download", tm.Download},
	} {
		row("timings", p.name+"_p50_ms", trimFloat(p.val.P50, 3))
		row("timings", p.name+"_p95_ms", trimFloat(p.val.P95, 3))
		row("timings", p.name+"_p99_ms", trimFloat(p.val.P99, 3))
	}

	row("environment", "os", e.OS)
	row("environment", "arch", e.Arch)
	row("environment", "cpus", strconv.Itoa(e.CPUs))
	row("environment", "gomaxprocs", strconv.Itoa(e.GOMAXPROCS))
	row("environment", "kernel", e.Kernel)
	row("environment", "go_version", e.GoVersion)
	row("environment", "memory_total_bytes", strconv.FormatUint(e.MemoryTotalBytes, 10))
	row("environment", "memory_available_bytes", strconv.FormatUint(e.MemoryAvailableBytes, 10))
	row("environment", "process_cpu_percent", trimFloat(e.ProcessCPUPercent, 2))
	row("environment", "max_rss_bytes", strconv.FormatUint(e.MaxRSSBytes, 10))
	row("environment", "goroutines_max", strconv.Itoa(e.GoroutinesMax))

	row("verdict", "status", v.Status)
	row("verdict", "title", v.Title)
	row("verdict", "message", v.Message)
	row("verdict", "breaking_rate", trimFloat(v.BreakingRate, 2))
	for i, rec := range v.Recommendations {
		row("verdict", fmt.Sprintf("recommendation_%d", i+1), rec)
	}

	cw.Flush()
	return cw.Error()
}
