// Package model defines the data shared by the load engine, the live UI and
// the web report.
package model

import "time"

const SchemaVersion = 1

// Report is the complete result of a load test. It is intentionally
// serializable to JSON so it can be exported, diffed and served.
type Report struct {
	SchemaVersion int             `json:"schema_version"`
	Meta          Meta            `json:"meta"`
	Summary       Summary         `json:"summary"`
	Latency       LatencyStats    `json:"latency"`
	Distribution  []LatencyBucket `json:"latency_distribution"`
	StatusCodes   []StatusCount   `json:"status_codes"`
	Errors        []ErrorCount    `json:"errors"`
	Timeline      []TimelinePoint `json:"timeline"`
	Timings       Timings         `json:"timings"`
	Thresholds    Thresholds      `json:"thresholds"`
	Verdict       Verdict         `json:"verdict"`
	Environment   Environment     `json:"environment"`
	Comparison    *Comparison     `json:"comparison,omitempty"`
}

// Comparison describes how a run changed relative to a saved baseline.
type Comparison struct {
	BaselineVersion string             `json:"baseline_version"`
	BaselineAt      time.Time          `json:"baseline_at"`
	TargetChanged   bool               `json:"target_changed"`
	Metrics         []ComparisonMetric `json:"metrics"`
	Regressions     []string           `json:"regressions,omitempty"`
	Improvements    []string           `json:"improvements,omitempty"`
}

// ComparisonMetric is one row of a baseline comparison.
type ComparisonMetric struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Unit         string  `json:"unit"`
	Baseline     float64 `json:"baseline"`
	Current      float64 `json:"current"`
	DeltaPercent float64 `json:"delta_percent,omitempty"`
	DeltaPoints  float64 `json:"delta_points,omitempty"`
	Better       string  `json:"better"`
	Regressed    bool    `json:"regressed,omitempty"`
	Improved     bool    `json:"improved,omitempty"`
}

// PhaseStats are percentiles in milliseconds for one HTTP phase.
type PhaseStats struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// Timings breaks the request latency into its network phases. Phases that do
// not apply to a request (DNS or TLS on reused connections) are recorded only
// when they actually happen, so their sample count may be lower.
type Timings struct {
	DNS           PhaseStats `json:"dns"`
	Connect       PhaseStats `json:"connect"`
	TLS           PhaseStats `json:"tls"`
	TTFB          PhaseStats `json:"ttfb"`
	Download      PhaseStats `json:"download"`
	ReusedPercent float64    `json:"reused_percent"`
	Samples       uint64     `json:"samples"`
}

// Meta describes what was tested and how.
type Meta struct {
	Tool            string      `json:"tool"`
	Version         string      `json:"version"`
	Target          string      `json:"target"`
	Method          string      `json:"method"`
	Mode            string      `json:"mode"`
	StartedAt       time.Time   `json:"started_at"`
	FinishedAt      time.Time   `json:"finished_at"`
	PlannedDuration int64       `json:"planned_duration_ms"`
	Duration        int64       `json:"duration_ms"`
	RateStart       float64     `json:"rate_start"`
	RateMax         float64     `json:"rate_max"`
	Workers         int         `json:"workers"`
	MaxWorkers      int         `json:"max_workers"`
	VUs             int         `json:"vus,omitempty"`
	ThinkTimeMs     int64       `json:"think_time_ms,omitempty"`
	HeaderNames     []string    `json:"header_names"`
	BodyBytes       int         `json:"body_bytes"`
	TimeoutMs       int64       `json:"timeout_ms"`
	Insecure        bool        `json:"insecure"`
	Interrupted     bool        `json:"interrupted"`
	AutoStopped     bool        `json:"auto_stopped"`
	StopReason      string      `json:"stop_reason,omitempty"`
	Stages          []StageInfo `json:"stages,omitempty"`
}

// StageInfo describes one step of a staged profile.
type StageInfo struct {
	Rate       float64 `json:"rate"`
	DurationMs int64   `json:"duration_ms"`
}

// Summary holds the headline numbers.
type Summary struct {
	Requests    uint64  `json:"requests"`
	Rate        float64 `json:"rate"`
	Throughput  float64 `json:"throughput"`
	SuccessRate float64 `json:"success_rate"`
	ErrorRate   float64 `json:"error_rate"`
	BytesIn     uint64  `json:"bytes_in"`
	BytesOut    uint64  `json:"bytes_out"`
}

// LatencyStats are expressed in milliseconds.
type LatencyStats struct {
	Min  float64 `json:"min"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	P999 float64 `json:"p999"`
	Max  float64 `json:"max"`
}

// LatencyBucket is one bar of the latency distribution histogram.
type LatencyBucket struct {
	Label string `json:"label"`
	Count uint64 `json:"count"`
}

// StatusCount is the number of responses for an HTTP status code.
type StatusCount struct {
	Code    int     `json:"code"`
	Count   uint64  `json:"count"`
	Percent float64 `json:"percent"`
}

// ErrorCount groups identical transport/protocol errors.
type ErrorCount struct {
	Message string `json:"message"`
	Count   uint64 `json:"count"`
}

// TimelinePoint aggregates one second of the test. Latencies are in ms.
type TimelinePoint struct {
	T          float64 `json:"t"`
	Requests   uint64  `json:"requests"`
	Errors     uint64  `json:"errors"`
	Rate       float64 `json:"rate"`
	TargetRate float64 `json:"target_rate"`
	P50        float64 `json:"p50"`
	P95        float64 `json:"p95"`
	P99        float64 `json:"p99"`
	ErrorRate  float64 `json:"error_rate"`

	// Client-side pressure during that second, when available.
	ClientCPUPercent      float64 `json:"client_cpu_percent,omitempty"`
	ClientCPUSharePercent float64 `json:"client_cpu_share_percent,omitempty"`
	ClientRSSBytes        uint64  `json:"client_rss_bytes,omitempty"`
	ClientGoroutines      int     `json:"client_goroutines,omitempty"`
}

// Thresholds are the SLOs used to judge the run.
type Thresholds struct {
	MaxErrorRate float64       `json:"max_error_rate"`
	MaxP95Ms     float64       `json:"max_p95_ms"`
	MaxStatus    []StatusLimit `json:"max_status,omitempty"`
}

// StatusLimit caps how many responses with a given status code are tolerated.
type StatusLimit struct {
	Code int    `json:"code"`
	Max  uint64 `json:"max"`
}

// Verdict status values.
const (
	VerdictPass        = "pass"
	VerdictLimit       = "limit"
	VerdictFail        = "fail"
	VerdictInterrupted = "interrupted"
)

// Verdict is the human-readable conclusion of the run.
type Verdict struct {
	Status          string   `json:"status"`
	Title           string   `json:"title"`
	Message         string   `json:"message"`
	BreakingRate    float64  `json:"breaking_rate"`
	BreakingAtSec   float64  `json:"breaking_at_sec"`
	Recommendations []string `json:"recommendations"`
}

// Environment records the machine that generated the load and how much of it
// LoadForge itself consumed. All resource fields are optional: collectors are
// best-effort and may be unavailable on some platforms.
type Environment struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	CPUs       int    `json:"cpus"`
	GoVersion  string `json:"go_version"`
	GOMAXPROCS int    `json:"gomaxprocs,omitempty"`
	Kernel     string `json:"kernel,omitempty"`

	MemoryTotalBytes     uint64  `json:"memory_total_bytes,omitempty"`
	MemoryAvailableBytes uint64  `json:"memory_available_bytes,omitempty"`
	Load1                float64 `json:"load1,omitempty"`
	Load5                float64 `json:"load5,omitempty"`
	Load15               float64 `json:"load15,omitempty"`

	ProcessCPUUserMs       float64 `json:"process_cpu_user_ms,omitempty"`
	ProcessCPUSystemMs     float64 `json:"process_cpu_system_ms,omitempty"`
	ProcessCPUPercent      float64 `json:"process_cpu_percent,omitempty"`
	ProcessCPUSharePercent float64 `json:"process_cpu_share_percent,omitempty"`
	MaxRSSBytes            uint64  `json:"max_rss_bytes,omitempty"`

	GoroutinesMax int `json:"goroutines_max,omitempty"`
	GoroutinesEnd int `json:"goroutines_end,omitempty"`

	HeapAllocBytes  uint64  `json:"heap_alloc_bytes,omitempty"`
	HeapSysBytes    uint64  `json:"heap_sys_bytes,omitempty"`
	TotalAllocBytes uint64  `json:"total_alloc_bytes,omitempty"`
	NumGC           uint32  `json:"num_gc,omitempty"`
	GCPauseTotalMs  float64 `json:"gc_pause_total_ms,omitempty"`
	GCCPUFraction   float64 `json:"gc_cpu_fraction,omitempty"`

	BatteryPercent      float64 `json:"battery_percent,omitempty"`
	BatteryTemperatureC float64 `json:"battery_temperature_c,omitempty"`
	Network             string  `json:"network,omitempty"`

	// ClientBottleneck is set when the load generator itself looks saturated,
	// so the reader does not blame the target for client-side limits.
	ClientBottleneck string `json:"client_bottleneck,omitempty"`
}

// Progress is a live snapshot consumed by the terminal UI.
type Progress struct {
	Elapsed     time.Duration
	Total       time.Duration
	Requests    uint64
	Rate        float64
	TargetRate  float64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	SuccessRate float64
	ErrorRate   float64
	Stopping    bool
}
