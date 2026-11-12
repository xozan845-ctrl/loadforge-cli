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
	Thresholds    Thresholds      `json:"thresholds"`
	Verdict       Verdict         `json:"verdict"`
	Environment   Environment     `json:"environment"`
}

// Meta describes what was tested and how.
type Meta struct {
	Tool            string    `json:"tool"`
	Version         string    `json:"version"`
	Target          string    `json:"target"`
	Method          string    `json:"method"`
	Mode            string    `json:"mode"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	PlannedDuration int64     `json:"planned_duration_ms"`
	Duration        int64     `json:"duration_ms"`
	RateStart       float64   `json:"rate_start"`
	RateMax         float64   `json:"rate_max"`
	Workers         int       `json:"workers"`
	MaxWorkers      int       `json:"max_workers"`
	HeaderNames     []string  `json:"header_names"`
	BodyBytes       int       `json:"body_bytes"`
	TimeoutMs       int64     `json:"timeout_ms"`
	Insecure        bool      `json:"insecure"`
	Interrupted     bool      `json:"interrupted"`
	AutoStopped     bool      `json:"auto_stopped"`
	StopReason      string    `json:"stop_reason,omitempty"`
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
}

// Thresholds are the SLOs used to judge the run.
type Thresholds struct {
	MaxErrorRate float64 `json:"max_error_rate"`
	MaxP95Ms     float64 `json:"max_p95_ms"`
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

// Environment records the machine that generated the load.
type Environment struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	GoVersion string `json:"go_version"`
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
