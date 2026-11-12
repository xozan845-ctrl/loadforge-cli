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
