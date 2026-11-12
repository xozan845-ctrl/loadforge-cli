// Package tui renders the live terminal dashboard.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/xozan845-ctrl/loadforge-cli/internal/engine"
	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	accent = lipgloss.Color("#22d3ee")
	green  = lipgloss.Color("#34d399")
	amber  = lipgloss.Color("#fbbf24")
	red    = lipgloss.Color("#fb7185")
	muted  = lipgloss.Color("#94a3b8")
	dim    = lipgloss.Color("#334155")
	border = lipgloss.Color("#1e293b")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	subStyle   = lipgloss.NewStyle().Foreground(muted)
	labelStyle = lipgloss.NewStyle().Foreground(muted)
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0"))
	okStyle    = lipgloss.NewStyle().Foreground(green).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(amber).Bold(true)
	badStyle   = lipgloss.NewStyle().Foreground(red).Bold(true)
	fillStyle  = lipgloss.NewStyle().Foreground(accent)
	emptyStyle = lipgloss.NewStyle().Foreground(dim)
)

// LiveState is the thread-safe snapshot shared by the engine and the TUI.
type LiveState struct {
	mu sync.RWMutex
	p  model.Progress
}

func (s *LiveState) Set(p model.Progress) {
	s.mu.Lock()
	s.p = p
	s.mu.Unlock()
}

func (s *LiveState) Get() model.Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.p
}

type resultMsg struct {
	report *model.Report
	err    error
}

type doneMsg resultMsg
type tickMsg time.Time

type modelState struct {
	cfg      engine.Config
	live     *LiveState
	done     <-chan resultMsg
	cancel   context.CancelFunc
	gate     *engine.Gate
	width    int
	height   int
	spinner  int
	report   *model.Report
	err      error
	stopping bool
}

// Run starts the live dashboard and the underlying load test. It returns the
// report even when the user interrupts the test.
func Run(cfg engine.Config) (*model.Report, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	live := &LiveState{}
	gate := engine.NewGate()
	cfg.Gate = gate
	startRate := cfg.Rate
	if cfg.Mode != engine.ModeConstant {
		startRate = cfg.StartRate
	}
	live.Set(model.Progress{Total: cfg.Duration, TargetRate: startRate})
	ch := make(chan resultMsg, 1)
	go func() {
		rep, err := engine.Run(ctx, cfg, live.Set)
		ch <- resultMsg{report: rep, err: err}
	}()

	m := &modelState{cfg: cfg, live: live, done: ch, cancel: cancel, width: 80, gate: gate}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.StartReturningModel()
	if errors.Is(err, tea.ErrInterrupted) {
		// SIGINT/SIGTERM outside raw mode: stop the engine, wait for in-flight
		// requests and keep the partial report instead of losing it.
		cancel()
		res := <-ch
		if res.err != nil {
			return nil, res.err
		}
		if res.report == nil {
			return nil, fmt.Errorf("la prueba terminó sin generar reporte")
		}
		return res.report, nil
	}
	if err != nil {
		return nil, err
	}
	fm, ok := final.(*modelState)
	if !ok {
		return nil, fmt.Errorf("estado de UI inesperado")
	}
	if fm.err != nil {
		return nil, fm.err
	}
	if fm.report == nil {
		return nil, fmt.Errorf("la prueba terminó sin generar reporte")
	}
	return fm.report, nil
}

func (m *modelState) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitCmd(m.done))
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func waitCmd(ch <-chan resultMsg) tea.Cmd {
	return func() tea.Msg { return doneMsg(<-ch) }
}

func (m *modelState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.spinner = (m.spinner + 1) % len(spinnerFrames)
		return m, tickCmd()
	case doneMsg:
		m.report = msg.report
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if m.stopping || m.report != nil {
				return m, tea.Quit
			}
			m.stopping = true
			m.cancel()
			return m, nil
		case "p":
			if m.gate != nil && m.report == nil {
				m.gate.Toggle()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *modelState) View() string {
	p := m.live.Get()
	width := m.width
	if width < 40 {
		width = 40
	}

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(titleStyle.Render("⚡ LOADFORGE"))
	b.WriteString("   ")
	b.WriteString(subStyle.Render(modeLabel(m.cfg)))
	if m.gate != nil && m.gate.Paused() {
		b.WriteString("   ")
		b.WriteString(warnStyle.Render("PAUSADO · p para reanudar"))
	}
	b.WriteString("\n\n  ")
	url := engine.SafeURL(m.cfg.URL)
	if len(url) > width-4 {
		url = url[:width-5] + "…"
	}
	b.WriteString(subStyle.Render(url))
	b.WriteString("\n\n  ")

	elapsed := p.Elapsed
	total := p.Total
	frac := 0.0
	if total > 0 {
		frac = float64(elapsed) / float64(total)
	}
	if frac > 1 {
		frac = 1
	}
	barWidth := width - 38
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 36 {
		barWidth = 36
	}
	spinner := spinnerFrames[m.spinner]
	if m.stopping {
		spinner = "◼"
	}
	b.WriteString(fillStyle.Render(spinner))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(fmt.Sprintf("%5.1fs", elapsed.Seconds())))
	b.WriteString(subStyle.Render(fmt.Sprintf(" / %.1fs  ", total.Seconds())))
	b.WriteString(renderBar(frac, barWidth))
	b.WriteString(" ")
	b.WriteString(subStyle.Render(fmt.Sprintf("%3.0f%%", frac*100)))
	b.WriteString("\n\n")

	status, statusStyle := health(p, m.cfg)
	writeStat(&b, "Peticiones", fmt.Sprintf("%s", formatInt(p.Requests)), "Ritmo",
		fmt.Sprintf("%.1f / %.0f req/s", p.Rate, p.TargetRate), width)
	writeStat(&b, "Latencia", fmt.Sprintf("P50 %s   P95 %s   P99 %s",
		formatDur(p.P50), formatDur(p.P95), formatDur(p.P99)), "", "", width)
	writeStat(&b, "Salud", fmt.Sprintf("%.1f%% éxito   %.2f%% errores",
		p.SuccessRate*100, p.ErrorRate*100), "Estado", statusStyle.Render(status), width)
	writeStat(&b, "Umbrales", thresholdText(m.cfg), "", "", width)

	b.WriteString("\n  ")
	if m.stopping {
		b.WriteString(warnStyle.Render("Deteniendo… esperando las peticiones en vuelo (Ctrl+C de nuevo para forzar)"))
	} else {
		b.WriteString(subStyle.Render("p pausar/reanudar · Ctrl+C detener y ver resultados parciales"))
	}
	b.WriteString("\n")
	return b.String()
}

func writeStat(b *strings.Builder, label, value, label2, value2 string, width int) {
	b.WriteString("  ")
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-12s", label)))
	b.WriteString(valueStyle.Render(value))
	if label2 != "" {
		pad := width - 16 - len(label) - len(value) - len(label2) - len(value2) - 6
		if pad < 2 {
			pad = 2
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(labelStyle.Render(label2 + " "))
		b.WriteString(valueStyle.Render(value2))
	}
	b.WriteString("\n")
}

func renderBar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	return fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

func health(p model.Progress, cfg engine.Config) (string, lipgloss.Style) {
	if p.Requests == 0 {
		return "midiendo", subStyle
	}
	bad := p.ErrorRate > cfg.MaxErrorRate ||
		(cfg.MaxP95 > 0 && p.P95 > cfg.MaxP95)
	if bad {
		return "SATURADO", badStyle
	}
	warn := p.ErrorRate > cfg.MaxErrorRate*0.7 ||
		(cfg.MaxP95 > 0 && p.P95 > time.Duration(float64(cfg.MaxP95)*0.7))
	if warn {
		return "DEGRADANDO", warnStyle
	}
	return "ESTABLE", okStyle
}

func modeLabel(cfg engine.Config) string {
	if len(cfg.Stages) > 0 {
		parts := make([]string, 0, len(cfg.Stages))
		for _, s := range cfg.Stages {
			parts = append(parts, fmt.Sprintf("%.0f", s.Rate))
		}
		return fmt.Sprintf("%s %s req/s · %s", cfg.Mode, strings.Join(parts, "→"), cfg.Duration)
	}
	if cfg.Mode == engine.ModeRamp {
		return fmt.Sprintf("ramp %.0f → %.0f req/s · %s", cfg.StartRate, cfg.MaxRate, cfg.Duration)
	}
	return fmt.Sprintf("constant %.0f req/s · %s", cfg.Rate, cfg.Duration)
}

func thresholdText(cfg engine.Config) string {
	errPart := fmt.Sprintf("errores ≤ %.1f%%", cfg.MaxErrorRate*100)
	if cfg.MaxP95 > 0 {
		return fmt.Sprintf("P95 ≤ %s   %s", formatDur(cfg.MaxP95), errPart)
	}
	return "P95 sin umbral   " + errPart
}

func formatDur(d time.Duration) string {
	if d <= 0 {
		return "0 ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
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
