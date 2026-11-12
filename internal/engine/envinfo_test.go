package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func TestDetectClientBottleneck(t *testing.T) {
	cfg := Config{Mode: ModeRamp, StartRate: 10, MaxRate: 90} // target ~50 req/s

	busy := model.Environment{ProcessCPUSharePercent: 85}
	detectClientBottleneck(&busy, cfg, 30)
	if busy.ClientBottleneck == "" {
		t.Fatal("expected a client bottleneck warning")
	}
	if !strings.Contains(busy.ClientBottleneck, "cliente") {
		t.Fatalf("unexpected message: %q", busy.ClientBottleneck)
	}

	relaxed := model.Environment{ProcessCPUSharePercent: 20}
	detectClientBottleneck(&relaxed, cfg, 30)
	if relaxed.ClientBottleneck != "" {
		t.Fatalf("unexpected warning at low CPU: %q", relaxed.ClientBottleneck)
	}

	onTarget := model.Environment{ProcessCPUSharePercent: 85}
	detectClientBottleneck(&onTarget, cfg, 48)
	if onTarget.ClientBottleneck != "" {
		t.Fatalf("unexpected warning when rate is on target: %q", onTarget.ClientBottleneck)
	}
}

func TestEnvSamplerAttachesTimelineSamples(t *testing.T) {
	s := newEnvSampler()
	time.Sleep(150 * time.Millisecond)
	s.tick(time.Now())

	points := []model.TimelinePoint{{T: 0}, {T: 1}}
	s.attachTimeline(points)
	if len(s.samples) == 0 {
		t.Skip("platform does not expose process CPU time")
	}
	if points[0].ClientCPUPercent == 0 && points[1].ClientCPUPercent == 0 {
		t.Fatalf("no client sample attached: %+v", points)
	}
}

func TestEnvSamplerReportsRuntime(t *testing.T) {
	s := newEnvSampler()
	time.Sleep(10 * time.Millisecond)
	env := s.build(time.Now())

	if env.OS == "" || env.Arch == "" {
		t.Fatalf("missing platform: %+v", env)
	}
	if env.CPUs < 1 || env.GOMAXPROCS < 1 {
		t.Fatalf("missing CPU data: %+v", env)
	}
	if env.GoVersion == "" {
		t.Fatalf("missing Go version: %+v", env)
	}
	if env.GoroutinesEnd < 1 || env.GoroutinesMax < 1 {
		t.Fatalf("missing goroutine data: %+v", env)
	}
}
