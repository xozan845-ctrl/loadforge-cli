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

// stageSpan is one interval of a staged profile.
type stageSpan struct {
	startRate float64
	endRate   float64
	dur       time.Duration
	startAt   time.Duration
	startHits float64
}

// stagesPacer implements step, spike and soak profiles. Step and soak hold a
// constant rate per stage; spike transitions linearly from the previous rate.
type stagesPacer struct {
	spans []stageSpan
	total time.Duration
}

var _ vegeta.Pacer = (*stagesPacer)(nil)

func newStagesPacer(mode string, stages []Stage) *stagesPacer {
	p := &stagesPacer{}
	at := time.Duration(0)
	hits := 0.0
	prev := 0.0
	for i, s := range stages {
		start := s.Rate
		if mode == ModeSpike && i > 0 {
			start = prev
		}
		p.spans = append(p.spans, stageSpan{
			startRate: start,
			endRate:   s.Rate,
			dur:       s.Duration,
			startAt:   at,
			startHits: hits,
		})
		if mode == ModeSpike {
			hits += (start + s.Rate) / 2 * s.Duration.Seconds()
		} else {
			hits += s.Rate * s.Duration.Seconds()
		}
		at += s.Duration
		prev = s.Rate
	}
	p.total = at
	return p
}

func (s stageSpan) hits() float64 {
	if s.dur <= 0 {
		return 0
	}
	return (s.startRate + s.endRate) / 2 * s.dur.Seconds()
}

func (p *stagesPacer) Rate(elapsed time.Duration) float64 {
	if len(p.spans) == 0 {
		return 0
	}
	if elapsed <= 0 {
		return p.spans[0].startRate
	}
	if elapsed >= p.total {
		return p.spans[len(p.spans)-1].endRate
	}
	for _, s := range p.spans {
		if elapsed < s.startAt+s.dur {
			if s.dur <= 0 {
				return s.endRate
			}
			frac := float64(elapsed-s.startAt) / float64(s.dur)
			return s.startRate + (s.endRate-s.startRate)*frac
		}
	}
	return p.spans[len(p.spans)-1].endRate
}

func (p *stagesPacer) Pace(elapsed time.Duration, hits uint64) (time.Duration, bool) {
	if len(p.spans) == 0 {
		return 0, true
	}
	if p.total > 0 && elapsed >= p.total {
		return 0, true
	}
	want := float64(hits + 1)

	for _, s := range p.spans {
		if want > s.startHits+s.hits()+1e-9 {
			continue
		}
		local := want - s.startHits
		if local < 0 {
			local = 0
		}
		target := s.startAt
		if s.dur <= 0 {
			return 0, true
		}
		if math.Abs(s.endRate-s.startRate) < 1e-12 {
			target += time.Duration(local / s.startRate * float64(time.Second))
		} else {
			a := (s.endRate - s.startRate) / (2 * s.dur.Seconds())
			b := s.startRate
			c := -local
			target += time.Duration((-b + math.Sqrt(b*b-4*a*c)) / (2 * a) * float64(time.Second))
		}
		if target > p.total {
			return 0, true
		}
		wait := target - elapsed
		if wait < 0 {
			wait = 0
		}
		return wait, false
	}
	return 0, true
}
