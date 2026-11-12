package engine

import (
	"context"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tsenart/vegeta/v12/lib"
)

func runWSVUs(ctx context.Context, cfg Config, gate *Gate) <-chan *vegeta.Result {
	out := make(chan *vegeta.Result, 256)
	end := time.Now().Add(cfg.Duration)
	msg := []byte(cfg.Message)
	if len(msg) == 0 {
		msg = []byte("ping")
	}

	send := func(res *vegeta.Result) bool {
		select {
		case out <- res:
			return true
		case <-ctx.Done():
			return false
		}
	}

	done := make(chan struct{}, cfg.VUs)
	for i := 0; i < cfg.VUs; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for ctx.Err() == nil && time.Now().Before(end) {
				if gate != nil {
					if err := gate.Wait(ctx); err != nil {
						return
					}
				}
				conn, _, err := websocket.DefaultDialer.DialContext(ctx, cfg.URL, nil)
				if err != nil {
					if !send(&vegeta.Result{Timestamp: time.Now(), Attack: "ws", Error: err.Error()}) {
						return
					}
					time.Sleep(200 * time.Millisecond)
					continue
				}
				for ctx.Err() == nil && time.Now().Before(end) {
					start := time.Now()
					res := &vegeta.Result{Timestamp: start, Attack: "ws", Method: "WS", URL: cfg.URL}
					_ = conn.SetWriteDeadline(time.Now().Add(cfg.Timeout))
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						res.Error = err.Error()
						res.Latency = time.Since(start)
						_ = send(res)
						break
					}
					_ = conn.SetReadDeadline(time.Now().Add(cfg.Timeout))
					_, data, err := conn.ReadMessage()
					res.Latency = time.Since(start)
					if err != nil {
						res.Error = err.Error()
						_ = send(res)
						break
					}
					res.Code = 200
					res.BytesOut = uint64(len(msg))
					res.BytesIn = uint64(len(data))
					if !send(res) {
						_ = conn.Close()
						return
					}
					if cfg.ThinkTime > 0 {
						timer := time.NewTimer(cfg.ThinkTime)
						select {
						case <-timer.C:
						case <-ctx.Done():
							timer.Stop()
							_ = conn.Close()
							return
						}
					}
				}
				_ = conn.Close()
			}
		}()
	}

	go func() {
		for i := 0; i < cfg.VUs; i++ {
			<-done
		}
		close(out)
	}()
	return out
}

func validWSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "ws" || u.Scheme == "wss") && u.Host != ""
}
