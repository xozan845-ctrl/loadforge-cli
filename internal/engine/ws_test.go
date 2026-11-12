package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRunWebSocketEcho(t *testing.T) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, msg)
		}
	}))
	defer srv.Close()

	cfg := Config{
		URL:          "ws" + strings.TrimPrefix(srv.URL, "http"),
		Mode:         ModeWS,
		VUs:          2,
		Duration:     300 * time.Millisecond,
		ThinkTime:    2 * time.Millisecond,
		Workers:      1,
		MaxWorkers:   1,
		Timeout:      2 * time.Second,
		Message:      "hola",
		MaxErrorRate: 0.5,
		MaxP95:       2 * time.Second,
	}
	report, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Summary.Requests == 0 {
		t.Fatal("expected websocket messages")
	}
	if report.Summary.SuccessRate != 1 {
		t.Fatalf("success = %v", report.Summary.SuccessRate)
	}
}
