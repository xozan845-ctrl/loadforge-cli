// Package engine runs the load test on top of Vegeta and aggregates the
// results into a portable report.
package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tsenart/vegeta/v12/lib"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// Mode values.
const (
	ModeConstant = "constant"
	ModeRamp     = "ramp"
)

// Config describes a load test.
type Config struct {
	URL       string
	Method    string
	Headers   http.Header
	Body      []byte
	Mode      string
	Rate      float64
	StartRate float64
	MaxRate   float64
	Duration  time.Duration

	Workers    int
	MaxWorkers int
	Timeout    time.Duration
	Insecure   bool
	MaxBody    int64

	MaxErrorRate float64
	MaxP95       time.Duration
	AutoStop     bool

	Version string
}

// Validate checks the config for obvious mistakes before hitting the network.
func (c *Config) Validate() error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("URL inválida: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("la URL debe empezar por http:// o https://")
	}
	if u.Host == "" {
		return fmt.Errorf("la URL no tiene host")
	}
	if c.Duration <= 0 {
		return fmt.Errorf("la duración debe ser mayor que cero")
	}
	if c.Mode != ModeConstant && c.Mode != ModeRamp {
		return fmt.Errorf("modo inválido %q: usa constant o ramp", c.Mode)
	}
	if c.Mode == ModeConstant && c.Rate <= 0 {
		return fmt.Errorf("el rate debe ser mayor que cero")
	}
	if c.Mode == ModeRamp {
		if c.StartRate <= 0 {
			return fmt.Errorf("start-rate debe ser mayor que cero")
		}
		if c.MaxRate < c.StartRate {
			return fmt.Errorf("max-rate debe ser mayor o igual que start-rate")
		}
	}
	if c.Workers < 1 {
		return fmt.Errorf("workers debe ser al menos 1")
	}
	if c.MaxWorkers < c.Workers {
		c.MaxWorkers = c.Workers
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("el timeout debe ser mayor que cero")
	}
	if c.MaxErrorRate < 0 || c.MaxErrorRate > 1 {
		return fmt.Errorf("max-error-rate debe estar entre 0 y 1")
	}
	if c.MaxP95 < 0 {
		return fmt.Errorf("max-p95 no puede ser negativo")
	}
	if c.Method == "" {
		c.Method = http.MethodGet
	}
	c.Method = strings.ToUpper(c.Method)
	if c.MaxBody <= 0 {
		c.MaxBody = 4096
	}
	return nil
}

// Run executes the test. ctx cancellation stops scheduling new requests, waits
// for in-flight requests and returns a partial report.
func Run(ctx context.Context, cfg Config, onProgress func(model.Progress)) (*model.Report, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	target := vegeta.Target{
		Method: cfg.Method,
		URL:    cfg.URL,
		Body:   cfg.Body,
		Header: cfg.Headers.Clone(),
	}
	if target.Header == nil {
		target.Header = http.Header{}
	}
	targeter := func(t *vegeta.Target) error {
		*t = target
		return nil
	}
	// Validate headers/URL once, before the attack starts.
	if _, err := target.Request(); err != nil {
		return nil, fmt.Errorf("no se pudo construir la petición: %w", err)
	}

	pacer := &linearPacer{
		start: cfg.Rate,
		end:   cfg.Rate,
		dur:   cfg.Duration,
	}
	if cfg.Mode == ModeRamp {
		pacer.start = cfg.StartRate
		pacer.end = cfg.MaxRate
	}

	attacker := vegeta.NewAttacker(
		vegeta.Workers(uint64(cfg.Workers)),
		vegeta.MaxWorkers(uint64(cfg.MaxWorkers)),
		vegeta.Client(newHTTPClient(cfg)),
		vegeta.MaxBody(cfg.MaxBody),
	)

	start := time.Now()
	agg := newAggregator(cfg, pacer, start)

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			attacker.Stop()
		case <-done:
		}
	}()
	defer close(done)

	results := attacker.Attack(targeter, pacer, cfg.Duration, "")

	var (
		lastProgress = time.Time{}
		checked      = 0
		autoStopped  = false
		stopReason   string
	)

	for res := range results {
		elapsed := time.Since(start)
		agg.add(res, elapsed)

		if cfg.AutoStop && !autoStopped {
			if len(agg.timeline) > checked {
				if triggered, reason := checkThresholds(agg.timeline, cfg); triggered {
					autoStopped = true
					stopReason = reason
					attacker.Stop()
				}
				checked = len(agg.timeline)
			}
		}

		if onProgress != nil && time.Since(lastProgress) >= 200*time.Millisecond {
			onProgress(agg.progress(elapsed, ctx.Err() != nil || autoStopped))
			lastProgress = time.Now()
		}
	}

	end := time.Now()
	agg.close(end.Sub(start))
	interrupted := ctx.Err() != nil && !autoStopped
	if onProgress != nil {
		onProgress(agg.progress(end.Sub(start), false))
	}

	return agg.report(interrupted, autoStopped, stopReason, end), nil
}

func newHTTPClient(cfg Config) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10000,
		MaxIdleConnsPerHost:   10000,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure, //nolint:gosec // explicit opt-in for test environments
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}
}

// checkThresholds inspects the most recent seconds as a rolling window so a
// couple of unlucky requests cannot abort a healthy test.
func checkThresholds(points []model.TimelinePoint, cfg Config) (bool, string) {
	if len(points) == 0 {
		return false, ""
	}
	start := len(points) - 5
	if start < 0 {
		start = 0
	}
	var (
		requests uint64
		errors   uint64
		p95Max   float64
		target   = points[len(points)-1].TargetRate
	)
	for _, p := range points[start:] {
		requests += p.Requests
		errors += p.Errors
		if p.P95 > p95Max {
			p95Max = p.P95
		}
	}
	if requests < 20 {
		return false, ""
	}
	stopErrors := cfg.MaxErrorRate * 2
	if stopErrors < 0.02 {
		stopErrors = 0.02
	}
	if errors >= 3 && ratio(errors, requests) > stopErrors {
		return true, fmt.Sprintf(
			"tasa de error %.1f%% supera el límite de parada %.1f%% en los últimos %d s a ~%.0f req/s",
			ratio(errors, requests)*100, stopErrors*100, len(points[start:]), target,
		)
	}
	if cfg.MaxP95 > 0 {
		stopP95 := float64(cfg.MaxP95.Milliseconds()) * 1.5
		if p95Max > stopP95 {
			return true, fmt.Sprintf(
				"P95 %.0f ms supera el límite de parada %.0f ms en los últimos %d s a ~%.0f req/s",
				p95Max, stopP95, len(points[start:]), target,
			)
		}
	}
	return false, ""
}

func ratio(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
