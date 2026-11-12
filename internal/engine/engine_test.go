package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func TestRunAgainstHealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := Config{
		URL:          srv.URL,
		Method:       http.MethodGet,
		Mode:         ModeConstant,
		Rate:         20,
		StartRate:    20,
		MaxRate:      20,
		Duration:     1200 * time.Millisecond,
		Workers:      4,
		MaxWorkers:   50,
		Timeout:      2 * time.Second,
		MaxErrorRate: 0.5,
		MaxP95:       2 * time.Second,
		Version:      "test",
	}

	progress := 0
	report, err := Run(context.Background(), cfg, func(model.Progress) { progress++ })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Summary.Requests < 10 {
		t.Fatalf("requests = %d, want >= 10", report.Summary.Requests)
	}
	if report.Summary.SuccessRate != 1 {
		t.Fatalf("success = %v, want 1", report.Summary.SuccessRate)
	}
	if len(report.Timeline) == 0 {
		t.Fatal("expected a non-empty timeline")
	}
	if report.Verdict.Status != model.VerdictPass {
		t.Fatalf("verdict = %q, want pass (%s)", report.Verdict.Status, report.Verdict.Message)
	}
	if progress == 0 {
		t.Fatal("expected progress callbacks")
	}
	if report.Environment.CPUs < 1 || report.Environment.GoVersion == "" {
		t.Fatalf("missing environment data: %+v", report.Environment)
	}
	if report.Environment.GoroutinesMax < 1 {
		t.Fatalf("missing goroutine peak: %+v", report.Environment)
	}
}

func TestRunCancellationReturnsPartialReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := Config{
		URL:          srv.URL,
		Method:       http.MethodGet,
		Mode:         ModeConstant,
		Rate:         10,
		StartRate:    10,
		MaxRate:      10,
		Duration:     5 * time.Second,
		Workers:      2,
		MaxWorkers:   20,
		Timeout:      2 * time.Second,
		MaxErrorRate: 0.5,
		MaxP95:       2 * time.Second,
		Version:      "test",
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	report, err := Run(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("cancellation took %v, want < 3s", elapsed)
	}
	if !report.Meta.Interrupted {
		t.Fatal("expected Meta.Interrupted")
	}
	if report.Summary.Requests == 0 {
		t.Fatal("expected some completed requests")
	}
	if report.Verdict.Status != model.VerdictInterrupted {
		t.Fatalf("verdict = %q, want interrupted", report.Verdict.Status)
	}
}
