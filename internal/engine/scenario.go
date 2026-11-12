package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tsenart/vegeta/v12/lib"
)

// Scenario is a multi-step user flow. Steps run in order; variables extracted
// from one response are available to the next ones.
type Scenario struct {
	Steps []ScenarioStep `json:"steps" yaml:"steps"`
}

// ScenarioStep is one request in a scenario.
type ScenarioStep struct {
	Name    string            `json:"name" yaml:"name"`
	Method  string            `json:"method" yaml:"method"`
	Path    string            `json:"path" yaml:"path"`
	URL     string            `json:"url" yaml:"url"`
	Headers map[string]string `json:"headers" yaml:"headers"`
	Body    string            `json:"body" yaml:"body"`
	Extract map[string]string `json:"extract" yaml:"extract"`
	PauseMs int               `json:"pause_ms" yaml:"pause_ms"`
}

var scenarioVar = regexp.MustCompile(`\{\{\.([A-Za-z0-9_]+)\}\}`)

type scenarioRunner struct {
	cfg  Config
	base string
	iter atomic.Uint64
}

func newScenarioRunner(cfg Config) *scenarioRunner {
	return &scenarioRunner{cfg: cfg, base: strings.TrimRight(cfg.URL, "/")}
}

func (r *scenarioRunner) stepURL(step ScenarioStep) (string, error) {
	raw := step.URL
	if raw == "" {
		raw = step.Path
	}
	if raw == "" {
		return "", fmt.Errorf("el paso %q no tiene path ni url", step.Name)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	return r.base + "/" + strings.TrimLeft(raw, "/"), nil
}

func (r *scenarioRunner) vars() map[string]string {
	n := r.iter.Add(1) - 1
	vars := map[string]string{
		"Iteration": fmt.Sprintf("%d", n+1),
		"RandomID":  randomID(),
	}
	if len(r.cfg.Rows) > 0 {
		for k, v := range r.cfg.Rows[int(n)%len(r.cfg.Rows)] {
			vars[k] = v
		}
	}
	return vars
}

func renderVars(tmpl string, vars map[string]string) string {
	return scenarioVar.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[3 : len(match)-2]
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}

func runScenarioVUs(ctx context.Context, cfg Config, client *http.Client, gate *Gate) <-chan *vegeta.Result {
	out := make(chan *vegeta.Result, 256)
	end := time.Now().Add(cfg.Duration)
	runner := newScenarioRunner(cfg)

	send := func(res *vegeta.Result) bool {
		select {
		case out <- res:
			return true
		case <-ctx.Done():
			return false
		}
	}

	var running atomic.Int64
	for i := 0; i < cfg.VUs; i++ {
		running.Add(1)
		go func() {
			defer running.Add(-1)
			for ctx.Err() == nil && time.Now().Before(end) {
				if gate != nil {
					if err := gate.Wait(ctx); err != nil {
						return
					}
				}
				vars := runner.vars()
				for _, step := range cfg.Scenario.Steps {
					if ctx.Err() != nil || time.Now().After(end) {
						return
					}
					res, ok := runScenarioStep(ctx, cfg, client, runner, step, vars)
					if !ok || !send(res) {
						return
					}
					if step.PauseMs > 0 {
						timer := time.NewTimer(time.Duration(step.PauseMs) * time.Millisecond)
						select {
						case <-timer.C:
						case <-ctx.Done():
							timer.Stop()
							return
						}
					}
				}
				if cfg.ThinkTime > 0 {
					timer := time.NewTimer(cfg.ThinkTime)
					select {
					case <-timer.C:
					case <-ctx.Done():
						timer.Stop()
						return
					}
				}
			}
		}()
	}

	go func() {
		for running.Load() > 0 {
			time.Sleep(10 * time.Millisecond)
		}
		close(out)
	}()
	return out
}

func runScenarioStep(ctx context.Context, cfg Config, client *http.Client, runner *scenarioRunner, step ScenarioStep, vars map[string]string) (*vegeta.Result, bool) {
	start := time.Now()
	res := &vegeta.Result{Timestamp: start, Attack: step.Name, Method: step.Method, URL: runner.base}
	if step.Method == "" {
		res.Method = http.MethodGet
	}

	targetURL, err := runner.stepURL(step)
	if err != nil {
		res.Error = err.Error()
		res.Latency = time.Since(start)
		return res, true
	}
	body := renderVars(step.Body, vars)
	req, err := http.NewRequestWithContext(ctx, res.Method, targetURL, strings.NewReader(body))
	if err != nil {
		res.Error = err.Error()
		res.Latency = time.Since(start)
		return res, true
	}
	for k, v := range step.Headers {
		req.Header.Set(k, renderVars(v, vars))
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		res.Latency = time.Since(start)
		return res, true
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxBody))
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	res.Latency = time.Since(start)
	res.Code = uint16(resp.StatusCode)
	res.BytesIn = uint64(len(data))
	res.BytesOut = uint64(len(body))
	res.Headers = resp.Header
	if res.Code < 200 || res.Code >= 400 {
		res.Error = resp.Status
	}
	if res.Code >= 200 && res.Code < 300 {
		extractVars(data, step.Extract, vars)
	}
	return res, true
}

func extractVars(data []byte, fields map[string]string, vars map[string]string) {
	if len(fields) == 0 {
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	for name, path := range fields {
		if v := nestedValue(payload, path); v != nil {
			switch value := v.(type) {
			case string:
				vars[name] = value
			case float64:
				vars[name] = fmt.Sprintf("%v", value)
			default:
				raw, _ := json.Marshal(value)
				vars[name] = string(raw)
			}
		}
	}
}
