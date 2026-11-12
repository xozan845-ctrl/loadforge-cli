package engine

import (
	"context"
	"sync"
	"time"
)

// Gate pauses and resumes request scheduling. In open-loop modes it shifts the
// pacer's clock so resuming does not fire a catch-up burst; in VU mode workers
// wait between requests.
type Gate struct {
	mu     sync.Mutex
	paused bool
	ch     chan struct{}
}

func NewGate() *Gate {
	return &Gate{ch: make(chan struct{})}
}

func (g *Gate) Pause() {
	g.mu.Lock()
	g.paused = true
	g.mu.Unlock()
}

func (g *Gate) Resume() {
	g.mu.Lock()
	if g.paused {
		g.paused = false
		close(g.ch)
		g.ch = make(chan struct{})
	}
	g.mu.Unlock()
}

func (g *Gate) Toggle() {
	g.mu.Lock()
	paused := g.paused
	g.mu.Unlock()
	if paused {
		g.Resume()
	} else {
		g.Pause()
	}
}

func (g *Gate) Paused() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.paused
}

// Wait blocks while the gate is paused. It returns ctx.Err() if the context is
// cancelled first.
func (g *Gate) Wait(ctx context.Context) error {
	for {
		g.mu.Lock()
		if !g.paused {
			g.mu.Unlock()
			return nil
		}
		ch := g.ch
		g.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// gatedPacer wraps a pacer and subtracts paused time from its clock.
type gatedPacer struct {
	inner vegetaPacer
	gate  *Gate

	mu       sync.Mutex
	pausing  bool
	pausedAt time.Time
	offset   time.Duration
}

// vegetaPacer mirrors the methods of vegeta.Pacer without importing it in
// gate.go, keeping the pause logic testable.
type vegetaPacer interface {
	Pace(time.Duration, uint64) (time.Duration, bool)
	Rate(time.Duration) float64
}

func newGatedPacer(inner vegetaPacer, gate *Gate) *gatedPacer {
	return &gatedPacer{inner: inner, gate: gate}
}

func (p *gatedPacer) adjust(elapsed time.Duration) (time.Duration, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	paused := p.gate.Paused()
	switch {
	case paused && !p.pausing:
		p.pausing = true
		p.pausedAt = time.Now()
	case !paused && p.pausing:
		p.offset += time.Since(p.pausedAt)
		p.pausing = false
	}
	adj := elapsed - p.offset
	if adj < 0 {
		adj = 0
	}
	return adj, paused
}

func (p *gatedPacer) Pace(elapsed time.Duration, hits uint64) (time.Duration, bool) {
	adj, paused := p.adjust(elapsed)
	if paused {
		return 200 * time.Millisecond, false
	}
	return p.inner.Pace(adj, hits)
}

func (p *gatedPacer) Rate(elapsed time.Duration) float64 {
	p.mu.Lock()
	offset := p.offset
	p.mu.Unlock()
	adj := elapsed - offset
	if adj < 0 {
		adj = 0
	}
	return p.inner.Rate(adj)
}
