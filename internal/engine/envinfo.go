package engine

import (
	"fmt"
	"runtime"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// envSample is a point-in-time view of the machine and the current process.
// Platforms fill in whatever they can; unsupported metrics stay zero.
type envSample struct {
	cpuUser time.Duration
	cpuSys  time.Duration
	cpuOK   bool

	maxRSSBytes uint64
	memTotal    uint64
	memAvail    uint64
	load1       float64
	load5       float64
	load15      float64
	kernel      string
}

// clientSample is one second of client-side pressure.
type clientSample struct {
	at         time.Duration
	cpuPercent float64
	cpuShare   float64
	rssBytes   uint64
	goroutines int
}

// envSampler measures the cost of generating the load. It takes a baseline
// before the attack, periodic samples during it and a final sample when the
// run ends.
type envSampler struct {
	start         envSample
	started       time.Time
	prev          envSample
	prevAt        time.Time
	maxGoroutines int
	samples       []clientSample
}

func newEnvSampler() *envSampler {
	start := readEnvSample()
	now := time.Now()
	s := &envSampler{
		start:   start,
		started: now,
		prev:    start,
		prevAt:  now,
	}
	s.sample()
	return s
}

// sample records a new goroutine high-water mark. It is cheap enough to call
// from the progress loop.
func (s *envSampler) sample() {
	if n := runtime.NumGoroutine(); n > s.maxGoroutines {
		s.maxGoroutines = n
	}
}

// tick records one client sample, computing CPU usage since the previous
// tick. It is called once per second during the attack.
func (s *envSampler) tick(now time.Time) {
	s.sample()

	cur := readEnvSample()
	if s.prev.cpuOK && cur.cpuOK {
		dt := now.Sub(s.prevAt)
		if dt >= 100*time.Millisecond {
			used := (cur.cpuUser + cur.cpuSys) - (s.prev.cpuUser + s.prev.cpuSys)
			if used >= 0 {
				pct := float64(used) / float64(dt) * 100
				parallelism := runtime.GOMAXPROCS(0)
				if parallelism < 1 {
					parallelism = runtime.NumCPU()
				}
				share := 0.0
				if parallelism > 0 {
					share = pct / float64(parallelism)
				}
				s.samples = append(s.samples, clientSample{
					at:         now.Sub(s.started),
					cpuPercent: pct,
					cpuShare:   share,
					rssBytes:   cur.maxRSSBytes,
					goroutines: runtime.NumGoroutine(),
				})
			}
		}
	}
	s.prev = cur
	s.prevAt = now
}

// attachTimeline copies the nearest client sample into each timeline bucket.
// Buckets without a close sample are left empty instead of inventing data.
func (s *envSampler) attachTimeline(points []model.TimelinePoint) {
	if len(s.samples) == 0 {
		return
	}
	for i := range points {
		want := time.Duration(points[i].T * float64(time.Second))
		best := -1
		var bestDiff time.Duration
		for j := range s.samples {
			diff := s.samples[j].at - want
			if diff < 0 {
				diff = -diff
			}
			if best < 0 || diff < bestDiff {
				best = j
				bestDiff = diff
			}
		}
		if best >= 0 && bestDiff <= 750*time.Millisecond {
			sample := s.samples[best]
			points[i].ClientCPUPercent = sample.cpuPercent
			points[i].ClientCPUSharePercent = sample.cpuShare
			points[i].ClientRSSBytes = sample.rssBytes
			points[i].ClientGoroutines = sample.goroutines
		}
	}
}

// build combines the runtime, the final process sample and the goroutine peak
// into the report environment block.
func (s *envSampler) build(end time.Time) model.Environment {
	final := readEnvSample()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	env := model.Environment{
		OS:                   runtime.GOOS,
		Arch:                 runtime.GOARCH,
		CPUs:                 runtime.NumCPU(),
		GoVersion:            runtime.Version(),
		GOMAXPROCS:           runtime.GOMAXPROCS(0),
		Kernel:               final.kernel,
		MemoryTotalBytes:     final.memTotal,
		MemoryAvailableBytes: final.memAvail,
		Load1:                final.load1,
		Load5:                final.load5,
		Load15:               final.load15,
		MaxRSSBytes:          final.maxRSSBytes,
		GoroutinesMax:        s.maxGoroutines,
		GoroutinesEnd:        runtime.NumGoroutine(),
		HeapAllocBytes:       ms.HeapAlloc,
		HeapSysBytes:         ms.HeapSys,
		TotalAllocBytes:      ms.TotalAlloc,
		NumGC:                ms.NumGC,
		GCPauseTotalMs:       float64(ms.PauseTotalNs) / float64(time.Millisecond),
		GCCPUFraction:        ms.GCCPUFraction,
	}

	if s.start.cpuOK && final.cpuOK {
		user := final.cpuUser - s.start.cpuUser
		sys := final.cpuSys - s.start.cpuSys
		env.ProcessCPUUserMs = float64(user) / float64(time.Millisecond)
		env.ProcessCPUSystemMs = float64(sys) / float64(time.Millisecond)

		if wall := end.Sub(s.started); wall > 0 {
			total := user + sys
			env.ProcessCPUPercent = float64(total) / float64(wall) * 100
			parallelism := runtime.GOMAXPROCS(0)
			if parallelism < 1 {
				parallelism = runtime.NumCPU()
			}
			if parallelism > 0 {
				env.ProcessCPUSharePercent = env.ProcessCPUPercent / float64(parallelism)
			}
		}
	}

	return env
}

// detectClientBottleneck flags runs where the generator used most of its CPU
// and still could not reach the target rate. This prevents attributing a
// client-side limit to the backend.
func detectClientBottleneck(env *model.Environment, cfg Config, rate float64) {
	if env.ProcessCPUSharePercent < 70 || rate <= 0 {
		return
	}
	target := cfg.Rate
	if cfg.Mode == ModeRamp {
		target = (cfg.StartRate + cfg.MaxRate) / 2
	}
	if target <= 0 || rate >= target*0.9 {
		return
	}
	env.ClientBottleneck = fmt.Sprintf(
		"El cliente usó ~%.0f%% de su capacidad de CPU y la tasa real (%.1f req/s) quedó por debajo de la objetivo (%.1f req/s). Parte de la degradación puede venir del equipo que generó la carga y no del backend.",
		env.ProcessCPUSharePercent, rate, target,
	)
}
