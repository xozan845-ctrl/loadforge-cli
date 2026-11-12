package engine

import (
	"testing"
	"time"
)

func TestConstantPacer(t *testing.T) {
	p := &linearPacer{start: 10, end: 10, dur: 10 * time.Second}
	if got := p.Rate(time.Second); got != 10 {
		t.Fatalf("Rate = %v, want 10", got)
	}
	wait, stop := p.Pace(0, 0)
	if stop || wait != 100*time.Millisecond {
		t.Fatalf("Pace(0,0) = %v,%v; want 100ms,false", wait, stop)
	}
	wait, stop = p.Pace(1*time.Second, 10)
	if stop || wait != 100*time.Millisecond {
		t.Fatalf("Pace(1s,10) = %v,%v; want 100ms,false", wait, stop)
	}
	if _, stop := p.Pace(10*time.Second, 100); !stop {
		t.Fatal("expected stop after duration")
	}
}

func TestStepPacer(t *testing.T) {
	p := newStagesPacer(ModeStep, []Stage{
		{Rate: 10, Duration: 2 * time.Second},
		{Rate: 50, Duration: 2 * time.Second},
	})
	if got := p.Rate(500 * time.Millisecond); got != 10 {
		t.Fatalf("Rate(0.5s) = %v, want 10", got)
	}
	if got := p.Rate(3 * time.Second); got != 50 {
		t.Fatalf("Rate(3s) = %v, want 50", got)
	}
	if _, stop := p.Pace(4*time.Second, 100); !stop {
		t.Fatal("expected stop after the last stage")
	}
	// The first hit at 10 req/s is due 100ms after start.
	wait, stop := p.Pace(0, 0)
	if stop || wait != 100*time.Millisecond {
		t.Fatalf("Pace(0,0) = %v,%v; want 100ms,false", wait, stop)
	}
}

func TestSpikePacerTransitions(t *testing.T) {
	p := newStagesPacer(ModeSpike, []Stage{
		{Rate: 10, Duration: time.Second},
		{Rate: 110, Duration: time.Second},
	})
	if got := p.Rate(1500 * time.Millisecond); got != 60 {
		t.Fatalf("Rate(1.5s) = %v, want 60", got)
	}
	if got := p.Rate(2 * time.Second); got != 110 {
		t.Fatalf("Rate(2s) = %v, want 110", got)
	}
}

func TestRampPacer(t *testing.T) {
	p := &linearPacer{start: 1, end: 101, dur: 10 * time.Second}
	if got := p.Rate(0); got != 1 {
		t.Fatalf("Rate(0) = %v", got)
	}
	if got := p.Rate(5 * time.Second); got != 51 {
		t.Fatalf("Rate(5s) = %v, want 51", got)
	}
	if got := p.Rate(10 * time.Second); got != 101 {
		t.Fatalf("Rate(10s) = %v, want 101", got)
	}
	// Cumulative hits at 10s: 1*10 + 0.5*10*10 = 510.
	wait, stop := p.Pace(9900*time.Millisecond, 500)
	if stop || wait <= 0 || wait > 20*time.Millisecond {
		t.Fatalf("Pace(9.9s,500) = %v,%v", wait, stop)
	}
}
