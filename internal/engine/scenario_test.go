package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestScenarioExtractsVariablesAndReportsSteps(t *testing.T) {
	var sawToken atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok-1"})
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer tok-1" {
			sawToken.Store(true)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := Config{
		URL:        srv.URL,
		Method:     http.MethodGet,
		Mode:       ModeVU,
		VUs:        2,
		Duration:   300 * time.Millisecond,
		ThinkTime:  2 * time.Millisecond,
		Workers:    1,
		MaxWorkers: 1,
		Timeout:    2 * time.Second,
		MaxBody:    4096,
		Scenario: &Scenario{Steps: []ScenarioStep{
			{Name: "login", Method: "POST", Path: "/login", Extract: map[string]string{"token": "token"}},
			{Name: "items", Method: "GET", Path: "/items",
				Headers: map[string]string{"Authorization": "Bearer {{.token}}"}},
		}},
	}
	report, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !sawToken.Load() {
		t.Fatal("second step did not receive the extracted token")
	}
	if len(report.Steps) != 2 {
		t.Fatalf("steps = %+v", report.Steps)
	}
	for _, step := range report.Steps {
		if step.Requests == 0 || step.ErrorRate != 0 {
			t.Fatalf("step %q = %+v", step.Name, step)
		}
	}
}
