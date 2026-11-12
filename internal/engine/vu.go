package engine

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/tsenart/vegeta/v12/lib"
)

// zeroPacer is a placeholder for closed-loop (VU) mode, where arrivals are
// driven by users instead of a target rate.
type zeroPacer struct{}

func (zeroPacer) Pace(time.Duration, uint64) (time.Duration, bool) { return 0, false }
func (zeroPacer) Rate(time.Duration) float64                       { return 0 }

// runVUs runs a closed model: each virtual user sends a request, reads the
// response, waits the think time and repeats until the duration ends.
func runVUs(ctx context.Context, cfg Config, builder *targetBuilder, client *http.Client, gate *Gate) <-chan *vegeta.Result {
	out := make(chan *vegeta.Result, 256)
	end := time.Now().Add(cfg.Duration)
	var wg sync.WaitGroup

	send := func(res *vegeta.Result) bool {
		select {
		case out <- res:
			return true
		case <-ctx.Done():
			return false
		}
	}

	for i := 0; i < cfg.VUs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil && time.Now().Before(end) {
				if gate != nil {
					if err := gate.Wait(ctx); err != nil {
						return
					}
				}
				target, err := builder.build()
				start := time.Now()
				res := &vegeta.Result{
					Timestamp: start,
					Method:    cfg.Method,
					URL:       cfg.URL,
				}
				if err != nil {
					res.Error = err.Error()
					res.Latency = time.Since(start)
					if !send(res) {
						return
					}
					continue
				}

				req, err := target.Request()
				if err != nil {
					res.Error = err.Error()
					res.Latency = time.Since(start)
					if !send(res) {
						return
					}
					continue
				}

				resp, err := client.Do(req)
				if err != nil {
					res.Error = err.Error()
					res.Latency = time.Since(start)
				} else {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxBody))
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
					res.Latency = time.Since(start)
					res.Code = uint16(resp.StatusCode)
					res.BytesIn = uint64(len(body))
					res.BytesOut = uint64(len(target.Body))
					res.Headers = resp.Header
					if res.Code < 200 || res.Code >= 400 {
						res.Error = resp.Status
					}
				}
				if !send(res) {
					return
				}

				if cfg.ThinkTime > 0 {
					timer := time.NewTimer(cfg.ThinkTime)
					select {
					case <-timer.C:
					case <-ctx.Done():
						timer.Stop()
						return
					}
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
