package engine

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	hdr "github.com/HdrHistogram/hdrhistogram-go"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// timingHeader carries per-request phase measurements from the transport to
// the aggregator. Vegeta exposes the response headers in its Result, so this
// avoids changing its API.
const timingHeader = "X-Loadforge-Trace"

// timingTransport measures DNS, TCP, TLS and TTFB for every request.
type timingTransport struct {
	base http.RoundTripper
}

func (t *timingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		dnsStart, connectStart, tlsStart time.Time
		dns, connect, tlsDur             time.Duration
		firstByte                        time.Time
		reused                           bool
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(httptrace.DNSDoneInfo) {
			if dns == 0 && !dnsStart.IsZero() {
				dns = time.Since(dnsStart)
			}
		},
		ConnectStart: func(_, _ string) {
			if connectStart.IsZero() {
				connectStart = time.Now()
			}
		},
		ConnectDone: func(_, _ string, err error) {
			if connect == 0 && err == nil && !connectStart.IsZero() {
				connect = time.Since(connectStart)
			}
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if tlsDur == 0 && err == nil && !tlsStart.IsZero() {
				tlsDur = time.Since(tlsStart)
			}
		},
		GotConn:              func(info httptrace.GotConnInfo) { reused = info.Reused },
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}

	start := time.Now()
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if !firstByte.IsZero() {
		ttfb := firstByte.Sub(start)
		resp.Header.Set(timingHeader, fmt.Sprintf("%d,%d,%d,%d,%t",
			dns.Nanoseconds(), connect.Nanoseconds(), tlsDur.Nanoseconds(), ttfb.Nanoseconds(), reused))
	}
	return resp, nil
}

func (a *aggregator) recordTiming(raw string, latency time.Duration) {
	parts := strings.Split(raw, ",")
	if len(parts) != 5 {
		return
	}
	values := make([]int64, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseInt(parts[i], 10, 64)
		if err != nil {
			return
		}
		values[i] = v
	}

	a.timingSamples++
	a.recordPhase(a.tDNS, values[0])
	a.recordPhase(a.tConnect, values[1])
	a.recordPhase(a.tTLS, values[2])
	a.recordPhase(a.tTTFB, values[3])

	ttfb := time.Duration(values[3])
	if download := latency - ttfb; download > 0 {
		a.recordPhase(a.tDownload, download.Nanoseconds())
	}
	if parts[4] == "true" {
		a.reusedConns++
	}
}

func (a *aggregator) recordPhase(h *hdr.Histogram, ns int64) {
	if ns <= 0 {
		return
	}
	us := ns / 1000
	if us < 1 {
		us = 1
	}
	if us > a.maxUs {
		us = a.maxUs
	}
	_ = h.RecordValue(us)
}

func phaseStats(h *hdr.Histogram) model.PhaseStats {
	return model.PhaseStats{
		P50: usToMs(h.ValueAtQuantile(50)),
		P95: usToMs(h.ValueAtQuantile(95)),
		P99: usToMs(h.ValueAtQuantile(99)),
	}
}
