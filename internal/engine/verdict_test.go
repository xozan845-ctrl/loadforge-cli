package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func baseVerdictConfig() Config {
	return Config{
		Mode:         ModeRamp,
		StartRate:    1,
		MaxRate:      50,
		MaxErrorRate: 0.05,
		MaxP95:       500 * time.Millisecond,
	}
}

func TestVerdictPassWithoutBreakingPoint(t *testing.T) {
	cfg := baseVerdictConfig()
	summary := model.Summary{Requests: 100, SuccessRate: 1, ErrorRate: 0}
	lat := model.LatencyStats{P50: 10, P95: 50, P99: 80}
	timeline := []model.TimelinePoint{
		{T: 0, Requests: 20, TargetRate: 10, P95: 40},
		{T: 1, Requests: 20, TargetRate: 20, P95: 50},
	}
	v := buildVerdict(cfg, summary, lat, timeline, nil, nil, false, false, "")
	if v.Status != model.VerdictPass {
		t.Fatalf("status = %q, want pass (%s)", v.Status, v.Message)
	}
}

func TestVerdictLimit(t *testing.T) {
	cfg := baseVerdictConfig()
	summary := model.Summary{Requests: 300, SuccessRate: 0.8, ErrorRate: 0.2}
	lat := model.LatencyStats{P50: 10, P95: 50, P99: 80}
	timeline := []model.TimelinePoint{
		{T: 0, Requests: 100, TargetRate: 10, P95: 40},
		{T: 1, Requests: 100, TargetRate: 20, P95: 50},
		{T: 2, Requests: 100, Errors: 20, TargetRate: 30, P95: 50, ErrorRate: 0.2},
	}
	v := buildVerdict(cfg, summary, lat, timeline, nil, nil, false, false, "")
	if v.Status != model.VerdictLimit {
		t.Fatalf("status = %q, want limit (%s)", v.Status, v.Message)
	}
	if v.BreakingRate != 20 {
		t.Fatalf("breaking rate = %v, want 20", v.BreakingRate)
	}
}

func TestVerdictAuthFailure(t *testing.T) {
	cfg := baseVerdictConfig()
	summary := model.Summary{Requests: 100, SuccessRate: 0, ErrorRate: 1}
	lat := model.LatencyStats{P95: 10}
	statuses := []model.StatusCount{{Code: 401, Count: 100, Percent: 1}}
	v := buildVerdict(cfg, summary, lat, nil, statuses, nil, false, false, "")
	if v.Status != model.VerdictFail || v.Title != "Autenticación rechazada" {
		t.Fatalf("verdict = %q / %q", v.Status, v.Title)
	}
}

func TestVerdictMissingEndpoint(t *testing.T) {
	cfg := baseVerdictConfig()
	summary := model.Summary{Requests: 100, SuccessRate: 0, ErrorRate: 1}
	lat := model.LatencyStats{P95: 10}
	statuses := []model.StatusCount{{Code: 404, Count: 100, Percent: 1}}
	v := buildVerdict(cfg, summary, lat, nil, statuses, nil, false, false, "")
	if v.Status != model.VerdictFail || v.Title != "Endpoint no encontrado" {
		t.Fatalf("verdict = %q / %q", v.Status, v.Title)
	}
	if len(v.Recommendations) == 0 || !strings.Contains(v.Recommendations[0], "404") {
		t.Fatalf("missing 404 recommendation: %v", v.Recommendations)
	}
}

func TestVerdictNoResponses(t *testing.T) {
	cfg := baseVerdictConfig()
	v := buildVerdict(cfg, model.Summary{}, model.LatencyStats{}, nil, nil, nil, false, false, "")
	if v.Status != model.VerdictFail || v.Title != "Sin respuestas" {
		t.Fatalf("verdict = %q / %q", v.Status, v.Title)
	}
}

func TestVerdictConstantOverThreshold(t *testing.T) {
	cfg := baseVerdictConfig()
	cfg.Mode = ModeConstant
	cfg.Rate = 50
	summary := model.Summary{Requests: 100, SuccessRate: 0.9, ErrorRate: 0.1}
	lat := model.LatencyStats{P95: 600}
	timeline := []model.TimelinePoint{
		{T: 0, Requests: 100, Errors: 10, TargetRate: 50, P95: 600, ErrorRate: 0.1},
	}
	v := buildVerdict(cfg, summary, lat, timeline, nil, nil, false, false, "")
	if v.Status != model.VerdictFail || v.Title != "Carga no sostenida" {
		t.Fatalf("verdict = %q / %q", v.Status, v.Title)
	}
}
