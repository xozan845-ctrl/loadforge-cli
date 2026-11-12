package engine

import (
	"fmt"
	"sort"
	"time"

	hdr "github.com/HdrHistogram/hdrhistogram-go"
	"github.com/tsenart/vegeta/v12/lib"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

var (
	distBounds = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	distLabels = []string{
		"< 5 ms", "5–10 ms", "10–25 ms", "25–50 ms", "50–100 ms",
		"100–250 ms", "250–500 ms", "0.5–1 s", "1–2.5 s", "2.5–5 s",
		"5–10 s", "> 10 s",
	}
)

type bucket struct {
	requests uint64
	errors   uint64
	bytesIn  uint64
	bytesOut uint64
	hist     *hdr.Histogram
}

func newBucket(maxUs int64) *bucket {
	h := hdr.New(1, maxUs, 3)
	return &bucket{hist: h}
}

func (b *bucket) add(res *vegeta.Result, maxUs int64) {
	b.requests++
	if res.Code < 200 || res.Code >= 400 || res.Error != "" {
		b.errors++
	}
	b.bytesIn += res.BytesIn
	b.bytesOut += res.BytesOut
	v := res.Latency.Microseconds()
	if v < 1 {
		v = 1
	}
	if v > maxUs {
		v = maxUs
	}
	_ = b.hist.RecordValue(v)
}

type aggregator struct {
	cfg   Config
	pacer vegeta.Pacer
	start time.Time
	maxUs int64
	hist  *hdr.Histogram
	dist  []uint64

	totalRequests uint64
	success       uint64
	bytesIn       uint64
	bytesOut      uint64
	totalLatency  int64

	codes map[int]uint64
	errs  map[string]uint64

	timeline []model.TimelinePoint
	cur      *bucket
	curIdx   int
}

func newAggregator(cfg Config, pacer vegeta.Pacer, start time.Time) *aggregator {
	maxUs := int64(10 * time.Minute / time.Microsecond)
	if cfg.Timeout*2 > 10*time.Minute {
		maxUs = int64(cfg.Timeout * 2 / time.Microsecond)
	}
	h := hdr.New(1, maxUs, 3)
	return &aggregator{
		cfg:   cfg,
		pacer: pacer,
		start: start,
		maxUs: maxUs,
		hist:  h,
		dist:  make([]uint64, len(distBounds)+1),
		codes: map[int]uint64{},
		errs:  map[string]uint64{},
	}
}

func (a *aggregator) add(res *vegeta.Result, elapsed time.Duration) {
	a.totalRequests++
	ok := res.Code >= 200 && res.Code < 400
	if ok {
		a.success++
	}
	a.bytesIn += res.BytesIn
	a.bytesOut += res.BytesOut
	us := res.Latency.Microseconds()
	if us < 1 {
		us = 1
	}
	a.totalLatency += us
	if us > a.maxUs {
		us = a.maxUs
	}
	_ = a.hist.RecordValue(us)
	a.dist[distIndex(float64(us)/1000)]++

	if res.Code > 0 {
		a.codes[int(res.Code)]++
	}
	if res.Error != "" {
		a.errs[truncate(res.Error, 300)]++
	} else if !ok && res.Code > 0 {
		a.errs[truncate(fmt.Sprintf("%d %s", res.Code, httpStatusText(int(res.Code))), 300)]++
	}

	idx := int(elapsed / time.Second)
	if idx < 0 {
		idx = 0
	}
	if a.cur == nil {
		a.curIdx = 0
		a.cur = newBucket(a.maxUs)
	}
	for a.curIdx < idx {
		start := time.Duration(a.curIdx) * time.Second
		end := start + time.Second
		if end > elapsed {
			end = elapsed
		}
		dur := end - start
		if dur <= 0 {
			dur = time.Millisecond
		}
		a.finalizeCurrent(dur, start)
		a.curIdx++
		a.cur = newBucket(a.maxUs)
	}
	a.cur.add(res, a.maxUs)
}

func (a *aggregator) finalizeCurrent(dur time.Duration, start time.Duration) {
	if a.cur == nil {
		return
	}
	rateDur := dur
	if rateDur < 100*time.Millisecond {
		rateDur = 100 * time.Millisecond
	}
	p := model.TimelinePoint{
		T:          start.Seconds(),
		Requests:   a.cur.requests,
		Errors:     a.cur.errors,
		Rate:       float64(a.cur.requests) / rateDur.Seconds(),
		TargetRate: a.pacer.Rate(start + dur/2),
		ErrorRate:  ratio(a.cur.errors, a.cur.requests),
	}
	if a.cur.requests > 0 {
		p.P50 = usToMs(a.cur.hist.ValueAtQuantile(50))
		p.P95 = usToMs(a.cur.hist.ValueAtQuantile(95))
		p.P99 = usToMs(a.cur.hist.ValueAtQuantile(99))
	}
	a.timeline = append(a.timeline, p)
}

func (a *aggregator) close(elapsed time.Duration) {
	if a.cur == nil {
		return
	}
	start := time.Duration(a.curIdx) * time.Second
	dur := elapsed - start
	if dur <= 0 {
		dur = time.Millisecond
	}
	a.finalizeCurrent(dur, start)
	a.cur = nil
}

func (a *aggregator) progress(elapsed time.Duration, stopping bool) model.Progress {
	p := model.Progress{
		Elapsed:     elapsed,
		Total:       a.cfg.Duration,
		Requests:    a.totalRequests,
		SuccessRate: ratio(a.success, a.totalRequests),
		ErrorRate:   ratio(a.totalRequests-a.success, a.totalRequests),
		Stopping:    stopping,
	}
	if a.totalRequests > 0 {
		p.P50 = usToDuration(a.hist.ValueAtQuantile(50))
		p.P95 = usToDuration(a.hist.ValueAtQuantile(95))
		p.P99 = usToDuration(a.hist.ValueAtQuantile(99))
	}
	if a.cur != nil && a.cur.requests > 0 {
		dur := elapsed - time.Duration(a.curIdx)*time.Second
		if dur < 100*time.Millisecond {
			dur = 100 * time.Millisecond
		}
		p.Rate = float64(a.cur.requests) / dur.Seconds()
	}
	p.TargetRate = a.pacer.Rate(elapsed)
	return p
}

func (a *aggregator) report(interrupted, autoStopped bool, stopReason string, end time.Time) *model.Report {
	dur := end.Sub(a.start)
	if dur < 0 {
		dur = 0
	}
	secs := dur.Seconds()
	summary := model.Summary{
		Requests:    a.totalRequests,
		SuccessRate: ratio(a.success, a.totalRequests),
		ErrorRate:   ratio(a.totalRequests-a.success, a.totalRequests),
		BytesIn:     a.bytesIn,
		BytesOut:    a.bytesOut,
	}
	if secs > 0 {
		summary.Rate = float64(a.totalRequests) / secs
		summary.Throughput = float64(a.success) / secs
	}

	lat := model.LatencyStats{}
	if a.totalRequests > 0 {
		lat.Min = usToMs(a.hist.Min())
		lat.Max = usToMs(a.hist.Max())
		lat.Mean = float64(a.totalLatency) / float64(a.totalRequests) / 1000
		lat.P50 = usToMs(a.hist.ValueAtQuantile(50))
		lat.P90 = usToMs(a.hist.ValueAtQuantile(90))
		lat.P95 = usToMs(a.hist.ValueAtQuantile(95))
		lat.P99 = usToMs(a.hist.ValueAtQuantile(99))
		lat.P999 = usToMs(a.hist.ValueAtQuantile(99.9))
	}

	statuses := make([]model.StatusCount, 0, len(a.codes))
	for code, count := range a.codes {
		statuses = append(statuses, model.StatusCount{
			Code:    code,
			Count:   count,
			Percent: ratio(count, a.totalRequests),
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Count == statuses[j].Count {
			return statuses[i].Code < statuses[j].Code
		}
		return statuses[i].Count > statuses[j].Count
	})

	errs := make([]model.ErrorCount, 0, len(a.errs))
	for msg, count := range a.errs {
		errs = append(errs, model.ErrorCount{Message: msg, Count: count})
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Count > errs[j].Count })
	if len(errs) > 20 {
		errs = errs[:20]
	}

	dist := make([]model.LatencyBucket, len(a.dist))
	for i, label := range distLabels {
		dist[i] = model.LatencyBucket{Label: label, Count: a.dist[i]}
	}

	thresholds := model.Thresholds{
		MaxErrorRate: a.cfg.MaxErrorRate,
		MaxP95Ms:     float64(a.cfg.MaxP95.Milliseconds()),
	}

	verdict := buildVerdict(a.cfg, summary, lat, a.timeline, statuses, errs, interrupted, autoStopped, stopReason)

	headers := make([]string, 0, len(a.cfg.Headers))
	for k := range a.cfg.Headers {
		headers = append(headers, k)
	}
	sort.Strings(headers)

	return &model.Report{
		SchemaVersion: model.SchemaVersion,
		Meta: model.Meta{
			Tool:            "loadforge",
			Version:         a.cfg.Version,
			Target:          sanitizeURL(a.cfg.URL),
			Method:          a.cfg.Method,
			Mode:            a.cfg.Mode,
			StartedAt:       a.start,
			FinishedAt:      end,
			PlannedDuration: a.cfg.Duration.Milliseconds(),
			Duration:        dur.Milliseconds(),
			RateStart:       a.cfg.StartRate,
			RateMax:         a.cfg.MaxRate,
			Workers:         a.cfg.Workers,
			MaxWorkers:      a.cfg.MaxWorkers,
			HeaderNames:     headers,
			BodyBytes:       len(a.cfg.Body),
			TimeoutMs:       a.cfg.Timeout.Milliseconds(),
			Insecure:        a.cfg.Insecure,
			Interrupted:     interrupted,
			AutoStopped:     autoStopped,
			StopReason:      stopReason,
		},
		Summary:      summary,
		Latency:      lat,
		Distribution: dist,
		StatusCodes:  statuses,
		Errors:       errs,
		Timeline:     a.timeline,
		Thresholds:   thresholds,
		Verdict:      verdict,
		Environment: model.Environment{
			OS:        runtimeOS(),
			Arch:      runtimeArch(),
			CPUs:      runtimeNumCPU(),
			GoVersion: runtimeVersion(),
		},
	}
}

func distIndex(ms float64) int {
	for i, b := range distBounds {
		if ms < b {
			return i
		}
	}
	return len(distBounds)
}

func usToMs(us int64) float64 { return float64(us) / 1000 }

func usToDuration(us int64) time.Duration { return time.Duration(us) * time.Microsecond }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
