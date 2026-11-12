// Package compare builds a regression comparison between two reports.
package compare

import (
	"fmt"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// Build compares current against base. Directions: "lower" metrics are better
// when they go down (latency, errors); "higher" metrics are better when they
// go up (throughput, success rate).
func Build(base, current *model.Report) model.Comparison {
	cmp := model.Comparison{
		BaselineVersion: base.Meta.Version,
		BaselineAt:      base.Meta.StartedAt,
		TargetChanged:   base.Meta.Target != current.Meta.Target,
	}

	add := func(key, label, unit string, better string, bv, cv float64) {
		m := model.ComparisonMetric{
			Key:      key,
			Label:    label,
			Unit:     unit,
			Baseline: bv,
			Current:  cv,
			Better:   better,
		}
		switch unit {
		case "%":
			m.DeltaPoints = (cv - bv) * 100
		default:
			if bv != 0 {
				m.DeltaPercent = (cv - bv) / bv * 100
			}
		}
		m.Regressed, m.Improved = classify(unit, better, m.DeltaPercent, m.DeltaPoints)
		if m.Regressed {
			cmp.Regressions = append(cmp.Regressions, label)
		}
		if m.Improved {
			cmp.Improvements = append(cmp.Improvements, label)
		}
		cmp.Metrics = append(cmp.Metrics, m)
	}

	add("latency_p95", "Latencia P95", "ms", "lower", base.Latency.P95, current.Latency.P95)
	add("latency_p99", "Latencia P99", "ms", "lower", base.Latency.P99, current.Latency.P99)
	add("latency_mean", "Latencia media", "ms", "lower", base.Latency.Mean, current.Latency.Mean)
	add("error_rate", "Tasa de error", "%", "lower", base.Summary.ErrorRate, current.Summary.ErrorRate)
	add("success_rate", "Tasa de éxito", "%", "higher", base.Summary.SuccessRate, current.Summary.SuccessRate)
	add("throughput", "Throughput", "req/s", "higher", base.Summary.Throughput, current.Summary.Throughput)
	add("rate", "Ritmo medio", "req/s", "higher", base.Summary.Rate, current.Summary.Rate)

	return cmp
}

func classify(unit, better string, deltaPercent, deltaPoints float64) (regressed, improved bool) {
	// Thresholds: 20% for latency, 10% for throughput, 1 percentage point for
	// rates. Percentages (rates) are compared in points.
	threshold := 10.0
	if unit == "%" {
		threshold = 1.0 // percentage points
	} else if unit == "ms" {
		threshold = 20.0
	}

	delta := deltaPercent
	if unit == "%" {
		delta = deltaPoints
	}
	switch better {
	case "lower":
		return delta > threshold, delta < -threshold
	default:
		return delta < -threshold, delta > threshold
	}
}

// FormatMetric renders a metric's values and delta for the terminal.
func FormatMetric(m model.ComparisonMetric) string {
	base := formatValue(m, m.Baseline)
	cur := formatValue(m, m.Current)
	delta := formatDelta(m)
	return fmt.Sprintf("%s → %s (%s)", base, cur, delta)
}

func formatValue(m model.ComparisonMetric, v float64) string {
	switch m.Unit {
	case "ms":
		if v >= 1000 {
			return fmt.Sprintf("%.2f s", v/1000)
		}
		return fmt.Sprintf("%.1f ms", v)
	case "%":
		return fmt.Sprintf("%.2f%%", v*100)
	default:
		return fmt.Sprintf("%.1f %s", v, m.Unit)
	}
}

func formatDelta(m model.ComparisonMetric) string {
	if m.Unit == "%" {
		return fmt.Sprintf("%+.2f pp", m.DeltaPoints)
	}
	return fmt.Sprintf("%+.1f%%", m.DeltaPercent)
}
