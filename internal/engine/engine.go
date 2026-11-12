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
	ModeStep     = "step"
	ModeSpike    = "spike"
	ModeSoak     = "soak"
	ModeVU       = "vu"
	ModeWS       = "ws"
)

// Stage is one step of a staged profile. For step and soak the rate is held
// constant; for spike the rate transitions linearly from the previous stage.
type Stage struct {
	Rate     float64
	Duration time.Duration
}

// Config describes a load test.
type Config struct {
	URL          string
	Method       string
	Headers      http.Header
	Body         []byte
	BodyTemplate string
	Rows         []map[string]string
	Mode         string
	Rate         float64
	StartRate    float64
	MaxRate      float64
	Stages       []Stage
	Duration     time.Duration

	Workers    int
	MaxWorkers int
	VUs        int
	ThinkTime  time.Duration
	Message    string
	Timeout    time.Duration
	Insecure   bool
	MaxBody    int64

	MaxErrorRate float64
	MaxP95       time.Duration
	MaxStatus    map[int]uint64
	Scenario     *Scenario
	OAuth        OAuth2Config
	Gate         *Gate
	AutoStop     bool
	Preflight    bool

	Version string
}

// Validate checks the config for obvious mistakes before hitting the network.
func (c *Config) Validate() error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("URL inválida: %w", err)
	}
	if c.Mode == ModeWS {
		if !validWSURL(c.URL) {
			return fmt.Errorf("--mode ws requiere una URL ws:// o wss://")
		}
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("la URL debe empezar por http:// o https://")
	}
	if u.Host == "" {
		return fmt.Errorf("la URL no tiene host")
	}
	if c.Duration <= 0 {
		return fmt.Errorf("la duración debe ser mayor que cero")
	}
	switch c.Mode {
	case ModeConstant:
		if c.Rate <= 0 {
			return fmt.Errorf("el rate debe ser mayor que cero")
		}
	case ModeRamp:
		if c.StartRate <= 0 {
			return fmt.Errorf("start-rate debe ser mayor que cero")
		}
		if c.MaxRate < c.StartRate {
			return fmt.Errorf("max-rate debe ser mayor o igual que start-rate")
		}
	case ModeVU:
		if c.VUs < 1 {
			return fmt.Errorf("--vus debe ser al menos 1")
		}
		if c.ThinkTime < 0 {
			return fmt.Errorf("--think-time no puede ser negativo")
		}
	case ModeWS:
		if c.VUs < 1 {
			return fmt.Errorf("--vus debe ser al menos 1")
		}
		if c.ThinkTime < 0 {
			return fmt.Errorf("--think-time no puede ser negativo")
		}
	case ModeStep, ModeSpike, ModeSoak:
		if len(c.Stages) == 0 {
			if c.Mode == ModeSoak && c.Rate > 0 && c.Duration > 0 {
				c.Stages = []Stage{{Rate: c.Rate, Duration: c.Duration}}
			} else {
				return fmt.Errorf("el modo %s requiere al menos un --stage RATE:DURATION", c.Mode)
			}
		}
		total := time.Duration(0)
		for i, s := range c.Stages {
			if s.Rate <= 0 {
				return fmt.Errorf("stage %d: el rate debe ser mayor que cero", i+1)
			}
			if s.Duration <= 0 {
				return fmt.Errorf("stage %d: la duración debe ser mayor que cero", i+1)
			}
			total += s.Duration
		}
		if total <= 0 {
			return fmt.Errorf("la duración total de los stages debe ser mayor que cero")
		}
		c.Duration = total
		c.StartRate = c.Stages[0].Rate
		c.MaxRate = c.PeakRate()
	default:
		return fmt.Errorf("modo inválido %q: usa constant, ramp, step, spike o soak", c.Mode)
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
	for code := range c.MaxStatus {
		if code < 100 || code > 599 {
			return fmt.Errorf("código HTTP inválido en --max-status: %d", code)
		}
	}
	if c.Scenario != nil {
		if c.Mode != ModeVU {
			return fmt.Errorf("--scenario requiere --mode vu")
		}
		if len(c.Scenario.Steps) == 0 {
			return fmt.Errorf("el escenario no tiene pasos")
		}
		for i, step := range c.Scenario.Steps {
			if step.URL == "" && step.Path == "" {
				return fmt.Errorf("paso %d (%s): falta path o url", i+1, step.Name)
			}
		}
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

// PeakRate returns the highest target rate of the profile.
func (c Config) PeakRate() float64 {
	switch c.Mode {
	case ModeConstant:
		return c.Rate
	case ModeRamp:
		return c.MaxRate
	default:
		peak := 0.0
		for _, s := range c.Stages {
			if s.Rate > peak {
				peak = s.Rate
			}
		}
		return peak
	}
}

// AverageRate returns the mean target rate over the whole profile. It is used
// to decide whether a run reached its expected load.
func (c Config) AverageRate() float64 {
	switch c.Mode {
	case ModeConstant:
		return c.Rate
	case ModeRamp:
		return (c.StartRate + c.MaxRate) / 2
	}

	totalHits := 0.0
	totalDur := 0.0
	prev := 0.0
	for i, s := range c.Stages {
		start := s.Rate
		if c.Mode == ModeSpike && i > 0 {
			start = prev
		}
		if c.Mode == ModeSpike {
			totalHits += (start + s.Rate) / 2 * s.Duration.Seconds()
		} else {
			totalHits += s.Rate * s.Duration.Seconds()
		}
		totalDur += s.Duration.Seconds()
		prev = s.Rate
	}
	if totalDur <= 0 {
		return 0
	}
	return totalHits / totalDur
}

// Run executes the test. ctx cancellation stops scheduling new requests, waits
// for in-flight requests and returns a partial report.
func Run(ctx context.Context, cfg Config, onProgress func(model.Progress)) (*model.Report, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var oauth *oauthSource
	if cfg.OAuth.TokenURL != "" {
		oauth = newOAuthSource(cfg.OAuth)
		if _, err := oauth.tokenFor(ctx); err != nil {
			return nil, err
		}
	}
	client := newHTTPClient(cfg, oauth)
	if cfg.Scenario == nil && cfg.Mode != ModeWS {
		if err := preflight(ctx, cfg, client); err != nil {
			return nil, err
		}
	}

	builder := newTargetBuilder(cfg)
	targeter := func(t *vegeta.Target) error {
		built, err := builder.build()
		if err != nil {
			return err
		}
		*t = built
		return nil
	}
	// Validate headers/URL once, before the attack starts.
	if cfg.Mode != ModeWS {
		if probe, err := builder.build(); err != nil {
			return nil, fmt.Errorf("no se pudo construir la petición: %w", err)
		} else if _, err := probe.Request(); err != nil {
			return nil, fmt.Errorf("no se pudo construir la petición: %w", err)
		}
	}

	var pacer vegeta.Pacer
	switch cfg.Mode {
	case ModeStep, ModeSpike, ModeSoak:
		pacer = newStagesPacer(cfg.Mode, cfg.Stages)
	case ModeVU:
		pacer = zeroPacer{}
	default:
		linear := &linearPacer{
			start: cfg.Rate,
			end:   cfg.Rate,
			dur:   cfg.Duration,
		}
		if cfg.Mode == ModeRamp {
			linear.start = cfg.StartRate
			linear.end = cfg.MaxRate
		}
		pacer = linear
	}
	if cfg.Gate != nil && cfg.Mode != ModeVU {
		pacer = newGatedPacer(pacer, cfg.Gate)
	}

	start := time.Now()
	agg := newAggregator(cfg, pacer, start)
	sampler := newEnvSampler()

	var results <-chan *vegeta.Result
	stopAttack := func() {}
	if cfg.Mode == ModeWS {
		wsCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stopAttack = cancel
		results = runWSVUs(wsCtx, cfg, cfg.Gate)
	} else if cfg.Mode == ModeVU {
		vuCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stopAttack = cancel
		if cfg.Scenario != nil {
			results = runScenarioVUs(vuCtx, cfg, client, cfg.Gate)
		} else {
			results = runVUs(vuCtx, cfg, builder, client, cfg.Gate)
		}
	} else {
		attacker := vegeta.NewAttacker(
			vegeta.Workers(uint64(cfg.Workers)),
			vegeta.MaxWorkers(uint64(cfg.MaxWorkers)),
			vegeta.Client(client),
			vegeta.MaxBody(cfg.MaxBody),
		)
		stopAttack = func() { attacker.Stop() }
		results = attacker.Attack(targeter, pacer, cfg.Duration, "")
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			stopAttack()
		case <-done:
		}
	}()
	defer close(done)

	var (
		lastProgress  = time.Time{}
		lastSample    = time.Time{}
		lastGoroutine = time.Time{}
		checked       = 0
		autoStopped   = false
		stopReason    string
	)

	for res := range results {
		elapsed := time.Since(start)
		agg.add(res, elapsed)

		if time.Since(lastGoroutine) >= 200*time.Millisecond {
			sampler.sample()
			lastGoroutine = time.Now()
		}
		if time.Since(lastSample) >= time.Second {
			sampler.tick(time.Now())
			lastSample = time.Now()
		}

		if cfg.AutoStop && !autoStopped {
			if len(agg.timeline) > checked {
				if triggered, reason := checkThresholds(agg.timeline, cfg); triggered {
					autoStopped = true
					stopReason = reason
					stopAttack()
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

	report := agg.report(interrupted, autoStopped, stopReason, end)
	report.Environment = sampler.build(end)
	sampler.attachTimeline(report.Timeline)
	detectClientBottleneck(&report.Environment, cfg, report.Summary.Rate)
	return report, nil
}

func newHTTPClient(cfg Config, oauth *oauthSource) *http.Client {
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
	var rt http.RoundTripper = &timingTransport{base: transport}
	if oauth != nil {
		rt = &oauthTransport{base: rt, source: oauth}
	}
	return &http.Client{
		Transport: rt,
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
