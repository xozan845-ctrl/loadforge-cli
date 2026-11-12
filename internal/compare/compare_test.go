package compare

import (
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func report(p95, errRate, success, throughput float64) *model.Report {
	return &model.Report{
		Meta: model.Meta{
			Version:   "test",
			Target:    "http://example.test/health",
			StartedAt: time.Now(),
		},
		Summary: model.Summary{ErrorRate: errRate, SuccessRate: success, Throughput: throughput, Rate: throughput},
		Latency: model.LatencyStats{P95: p95, P99: p95 * 1.2, Mean: p95 * 0.8},
	}
}

func TestBuildDetectsRegressions(t *testing.T) {
	base := report(40, 0.01, 1, 100)
	current := report(60, 0.03, 0.97, 80)

	cmp := Build(base, current)
	if len(cmp.Regressions) == 0 {
		t.Fatalf("expected regressions: %+v", cmp.Metrics)
	}
	if len(cmp.Improvements) != 0 {
		t.Fatalf("unexpected improvements: %v", cmp.Improvements)
	}

	byKey := map[string]model.ComparisonMetric{}
	for _, m := range cmp.Metrics {
		byKey[m.Key] = m
	}
	if !byKey["latency_p95"].Regressed {
		t.Fatal("P95 should be marked as regressed")
	}
	if !byKey["error_rate"].Regressed {
		t.Fatal("error rate should be marked as regressed")
	}
	if !byKey["throughput"].Regressed {
		t.Fatal("throughput should be marked as regressed")
	}
}

func TestBuildDetectsImprovements(t *testing.T) {
	base := report(80, 0.05, 0.95, 50)
	current := report(40, 0.01, 1, 100)

	cmp := Build(base, current)
	if len(cmp.Improvements) == 0 {
		t.Fatalf("expected improvements: %+v", cmp.Metrics)
	}
	if len(cmp.Regressions) != 0 {
		t.Fatalf("unexpected regressions: %v", cmp.Regressions)
	}
}

func TestTargetChanged(t *testing.T) {
	base := report(40, 0, 1, 100)
	current := report(40, 0, 1, 100)
	current.Meta.Target = "http://other.test/health"
	if !Build(base, current).TargetChanged {
		t.Fatal("expected target change to be flagged")
	}
}
