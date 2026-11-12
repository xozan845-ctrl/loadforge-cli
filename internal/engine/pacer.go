package engine

import (
	"math"
	"time"

	"github.com/tsenart/vegeta/v12/lib"
)

// linearPacer implements a linearly ramping rate. When start == end it behaves
// like a constant pacer. It computes the exact expected arrival time of each
// hit by integrating the rate curve, which avoids accumulating drift.
type linearPacer struct {
	start float64
	end   float64
	dur   time.Duration
}

var _ vegeta.Pacer = (*linearPacer)(nil)

func (p *linearPacer) durSeconds() float64 {
	if p.dur <= 0 {
		return 0
	}
	return p.dur.Seconds()
}

func (p *linearPacer) rateAt(elapsed time.Duration) float64 {
	if p.start == p.end {
		return p.start
	}
	if elapsed <= 0 {
		return p.start
	}
	d := p.durSeconds()
	if d <= 0 || elapsed.Seconds() >= d {
		return p.end
	}
	frac := elapsed.Seconds() / d
	return p.start + (p.end-p.start)*frac
}

// Pace returns how long to wait before sending the next hit (hits+1).
func (p *linearPacer) Pace(elapsed time.Duration, hits uint64) (time.Duration, bool) {
	if p.dur > 0 && elapsed >= p.dur {
		return 0, true
	}
	if p.start == p.end {
		if p.start <= 0 {
			return 0, false
		}
		target := time.Duration(float64(hits+1) / p.start * float64(time.Second))
		if target <= elapsed {
			return 0, false
		}
		return target - elapsed, false
	}

	// Cumulative hits at time t:
	//   H(t) = start*t + (end-start)/(2*dur) * t^2
	// Solve H(t) = hits+1 for the next hit time.
	a := (p.end - p.start) / (2 * p.durSeconds())
	b := p.start
	c := -float64(hits + 1)
	var target float64
	if math.Abs(a) < 1e-12 {
		target = -c / b
	} else {
		disc := b*b - 4*a*c
		if disc < 0 {
			disc = 0
		}
		target = (-b + math.Sqrt(disc)) / (2 * a)
	}
	if target <= 0 || math.IsNaN(target) || math.IsInf(target, 0) {
		return 0, false
	}
	wait := time.Duration(target*float64(time.Second)) - elapsed
	if wait < 0 {
		return 0, false
	}
	return wait, false
}

// Rate returns the instantaneous target rate at elapsed.
func (p *linearPacer) Rate(elapsed time.Duration) float64 {
	return p.rateAt(elapsed)
}
