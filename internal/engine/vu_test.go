package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func TestRunVUClosedLoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := Config{
		URL:          srv.URL,
		Method:       http.MethodGet,
		Mode:         ModeVU,
		VUs:          4,
		ThinkTime:    5 * time.Millisecond,
		Duration:     400 * time.Millisecond,
		Workers:      1,
		MaxWorkers:   1,
		Timeout:      2 * time.Second,
		MaxBody:      4096,
		MaxErrorRate: 0.5,
		MaxP95:       2 * time.Second,
	}
	report, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Summary.Requests < 5 {
		t.Fatalf("requests = %d, want at least 5", report.Summary.Requests)
	}
	if report.Summary.SuccessRate != 1 {
		t.Fatalf("success = %v", report.Summary.SuccessRate)
	}
	if report.Meta.VUs != 4 {
		t.Fatalf("vus = %d", report.Meta.VUs)
	}
	if report.Verdict.Status != model.VerdictPass {
		t.Fatalf("verdict = %q: %s", report.Verdict.Status, report.Verdict.Message)
	}
}
