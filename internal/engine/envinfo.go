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

// envSampler measures the cost of generating the load. It takes a baseline
// before the attack and a final sample when the run ends.
type envSampler struct {
	start         envSample
	started       time.Time
	maxGoroutines int
}

func newEnvSampler() *envSampler {
	s := &envSampler{
		start:   readEnvSample(),
		started: time.Now(),
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
