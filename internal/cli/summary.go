package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

var (
	cyan   = lipgloss.Color("#22d3ee")
	green  = lipgloss.Color("#34d399")
	amber  = lipgloss.Color("#fbbf24")
	red    = lipgloss.Color("#fb7185")
	muted  = lipgloss.Color("#94a3b8")
	faint  = lipgloss.Color("#64748b")
	ink    = lipgloss.Color("#e2e8f0")
	border = lipgloss.Color("#1b2a44")

	errorStyle = lipgloss.NewStyle().Foreground(red).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(amber).Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(green).Bold(true)
	labelStyle = lipgloss.NewStyle().Foreground(faint)
	valueStyle = lipgloss.NewStyle().Foreground(ink)
	mutedStyle = lipgloss.NewStyle().Foreground(muted)
	linkStyle  = lipgloss.NewStyle().Foreground(cyan).Bold(true).Underline(true)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 2)
)

func printSummary(r *model.Report, savedPath string) {
	width := summaryWidth()
	v := r.Verdict
	icon, color := verdictVisual(v.Status)

	fmt.Println()
	fmt.Println("  " + lipgloss.NewStyle().Bold(true).Foreground(color).Render(icon+"  "+strings.ToUpper(v.Title)))
	if v.Message != "" {
		msg := mutedStyle.Width(width - 4).Render(v.Message)
		fmt.Println("  " + strings.ReplaceAll(msg, "\n", "\n  "))
	}
	fmt.Println()

	pairs := [][2]string{
		{"Peticiones", formatInt(r.Summary.Requests)},
		{"Ritmo medio", fmt.Sprintf("%.1f req/s", r.Summary.Rate)},
		{"Éxito", fmt.Sprintf("%.2f%%", r.Summary.SuccessRate*100)},
		{"Errores", fmt.Sprintf("%.2f%%", r.Summary.ErrorRate*100)},
	}

	boxWidth := width - 2
	contentWidth := boxWidth - 4
	var rows []string
	if contentWidth >= 60 {
		colW := (contentWidth - 1) / 2
		for i := 0; i < len(pairs); i += 2 {
			left := lipgloss.NewStyle().Width(colW).Render(kv(pairs[i][0], pairs[i][1]))
			right := lipgloss.NewStyle().Width(colW).Render(kv(pairs[i+1][0], pairs[i+1][1]))
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
		}
	} else {
		for _, p := range pairs {
			rows = append(rows, kv(p[0], p[1]))
		}
	}

	narrow := contentWidth < 58
	if narrow {
		rows = append(rows, kv("Latencia", fmt.Sprintf("P50 %s · P95 %s",
			formatMs(r.Latency.P50), formatMs(r.Latency.P95))))
		rows = append(rows, kv("Cola", fmt.Sprintf("P99 %s · máx %s",
			formatMs(r.Latency.P99), formatMs(r.Latency.Max))))
		rows = append(rows, kv("Tráfico", fmt.Sprintf("%s ↓ · %s ↑",
			humanBytes(r.Summary.BytesIn), humanBytes(r.Summary.BytesOut))))
	} else {
		rows = append(rows, kv("Latencia", fmt.Sprintf("P50 %s · P90 %s · P95 %s · P99 %s",
			formatMs(r.Latency.P50), formatMs(r.Latency.P90), formatMs(r.Latency.P95), formatMs(r.Latency.P99))))
		rows = append(rows, kv("Transferencia", fmt.Sprintf("%s recibidos · %s enviados",
			humanBytes(r.Summary.BytesIn), humanBytes(r.Summary.BytesOut))))
	}

	if r.Meta.Mode == "ramp" {
		switch v.Status {
		case model.VerdictLimit:
			if narrow {
				rows = append(rows, kv("Capacidad", fmt.Sprintf("~%.0f req/s (límite)", v.BreakingRate)))
			} else {
				rows = append(rows, kv("Capacidad", fmt.Sprintf("~%.0f req/s sostenidos (límite detectado)", v.BreakingRate)))
			}
		case model.VerdictPass:
			if narrow {
				rows = append(rows, kv("Capacidad", fmt.Sprintf("hasta %.0f req/s", r.Meta.RateMax)))
			} else {
				rows = append(rows, kv("Capacidad", fmt.Sprintf("hasta %.0f req/s sin punto de quiebre", r.Meta.RateMax)))
			}
		}
	}
	if narrow {
		rows = append(rows, kv("Umbrales", thresholdsTextNarrow(r.Thresholds)))
	} else {
		rows = append(rows, kv("Umbrales", thresholdsText(r.Thresholds)))
	}
	rows = append(rows, kv("Entorno", environmentText(r.Environment, narrow)))
	rows = append(rows, kv("Cliente", clientText(r.Environment, narrow)))
	if resources := clientResourcesText(r.Environment, narrow); resources != "" {
		rows = append(rows, kv("Recursos", resources))
	}

	fmt.Println(boxStyle.Width(boxWidth).Render(strings.Join(rows, "\n")))
	fmt.Println()

	if r.Environment.ClientBottleneck != "" {
		wrapped := mutedStyle.Width(width - 4).Render(r.Environment.ClientBottleneck)
		fmt.Println("  " + warnStyle.Render("⚠ ") +
			strings.ReplaceAll(wrapped, "\n", "\n    "))
		fmt.Println()
	}

	if len(v.Recommendations) > 0 {
		fmt.Println("  " + labelStyle.Render("Recomendaciones"))
		max := 3
		for i, rec := range v.Recommendations {
			if i >= max {
				break
			}
			wrapped := mutedStyle.Width(width - 8).Render(rec)
			fmt.Println("  " + lipgloss.NewStyle().Foreground(cyan).Render("→") + " " +
				strings.ReplaceAll(wrapped, "\n", "\n    "))
		}
		if len(v.Recommendations) > max {
			fmt.Printf("    %s\n", faintStyle(fmt.Sprintf("… y %d más en el reporte web", len(v.Recommendations)-max)))
		}
		fmt.Println()
	}

	if savedPath != "" {
		fmt.Println("  " + okStyle.Render("✓") + " " + valueStyle.Render("JSON guardado en ") + mutedStyle.Render(savedPath))
		fmt.Println()
	}
}

func printReportURL(url string) {
	fmt.Println("  " + labelStyle.Render("Reporte web listo"))
	fmt.Println("  " + linkStyle.Render(url))
	fmt.Println("  " + mutedStyle.Render("Ctrl+C para cerrar el servidor"))
	fmt.Println()
}

func kv(label, value string) string {
	return labelStyle.Render(fmt.Sprintf("%-13s", label)) + " " + valueStyle.Render(value)
}

func verdictVisual(status string) (string, lipgloss.Color) {
	switch status {
	case model.VerdictPass:
		return "✔", green
	case model.VerdictLimit:
		return "▲", amber
	case model.VerdictFail:
		return "✕", red
	case model.VerdictInterrupted:
		return "◼", muted
	default:
		return "•", muted
	}
}

func summaryWidth() int {
	w := 84
	if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 {
		w = tw - 4
	}
	if w < 50 {
		w = 50
	}
	if w > 96 {
		w = 96
	}
	return w
}

func formatMs(v float64) string {
	if v <= 0 {
		return "0 ms"
	}
	if v >= 1000 {
		return fmt.Sprintf("%.2f s", v/1000)
	}
	if v >= 100 {
		return fmt.Sprintf("%.0f ms", v)
	}
	return fmt.Sprintf("%.1f ms", v)
}

func thresholdsText(t model.Thresholds) string {
	errPart := fmt.Sprintf("errores ≤ %.2f%%", t.MaxErrorRate*100)
	if t.MaxP95Ms > 0 {
		return fmt.Sprintf("P95 ≤ %s · %s", formatMs(t.MaxP95Ms), errPart)
	}
	return "P95 sin umbral · " + errPart
}

func environmentText(e model.Environment, narrow bool) string {
	parts := []string{e.OS + "/" + e.Arch}
	if e.CPUs > 0 {
		parts = append(parts, fmt.Sprintf("%d CPU", e.CPUs))
	}
	if !narrow && e.GOMAXPROCS > 0 && e.GOMAXPROCS != e.CPUs {
		parts = append(parts, fmt.Sprintf("GOMAXPROCS %d", e.GOMAXPROCS))
	}
	if e.MemoryTotalBytes > 0 {
		if narrow {
			parts = append(parts, humanBytes(e.MemoryTotalBytes))
		} else {
			parts = append(parts, humanBytes(e.MemoryTotalBytes)+" RAM")
		}
	}
	return strings.Join(parts, " · ")
}

func clientText(e model.Environment, narrow bool) string {
	if e.ProcessCPUPercent <= 0 {
		return "no disponible"
	}
	if narrow {
		cpu := fmt.Sprintf("CPU %.1f%%", e.ProcessCPUPercent)
		if e.ProcessCPUSharePercent > 0 {
			cpu += fmt.Sprintf(" (%.1f%%)", e.ProcessCPUSharePercent)
		}
		return cpu
	}
	cpu := fmt.Sprintf("CPU %.1f%% de un núcleo", e.ProcessCPUPercent)
	if e.ProcessCPUSharePercent > 0 {
		cpu += fmt.Sprintf(" · %.1f%% del equipo", e.ProcessCPUSharePercent)
	}
	return cpu
}

func clientResourcesText(e model.Environment, narrow bool) string {
	var parts []string
	if e.MaxRSSBytes > 0 {
		if narrow {
			parts = append(parts, humanBytes(e.MaxRSSBytes))
		} else {
			parts = append(parts, humanBytes(e.MaxRSSBytes)+" RSS")
		}
	}
	if e.GoroutinesMax > 0 {
		parts = append(parts, fmt.Sprintf("%d goroutines", e.GoroutinesMax))
	}
	if e.NumGC > 0 {
		parts = append(parts, fmt.Sprintf("GC %d×", e.NumGC))
	}
	return strings.Join(parts, " · ")
}

func thresholdsTextNarrow(t model.Thresholds) string {
	errPart := fmt.Sprintf("err ≤ %.2f%%", t.MaxErrorRate*100)
	if t.MaxP95Ms > 0 {
		return fmt.Sprintf("P95 ≤ %s · %s", formatMs(t.MaxP95Ms), errPart)
	}
	return errPart
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0 ms"
	}
	return formatMs(float64(d.Milliseconds()))
}

func formatInt(n uint64) string {
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

func faintStyle(s string) string {
	return lipgloss.NewStyle().Foreground(faint).Render(s)
}
