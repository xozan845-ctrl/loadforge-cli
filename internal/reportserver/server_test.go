package reportserver

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func testReport() *model.Report {
	return &model.Report{
		SchemaVersion: model.SchemaVersion,
		Meta: model.Meta{
			Tool:      "loadforge",
			Version:   "test",
			Target:    "http://example.test/api",
			Method:    "GET",
			Mode:      "ramp",
			StartedAt: time.Now(),
		},
		Summary:  model.Summary{Requests: 10, SuccessRate: 1},
		Verdict:  model.Verdict{Status: model.VerdictPass, Title: "OK"},
		Timeline: []model.TimelinePoint{{T: 0, Requests: 10, Rate: 10, TargetRate: 10, P50: 1, P95: 2, P99: 3}},
	}
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res.StatusCode, string(body)
}

func TestServeReportEndpoints(t *testing.T) {
	srv, err := Serve(testReport(), "127.0.0.1", 0, false)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	defer srv.Close()

	if code, body := get(t, srv.URL+"/api/report"); code != 200 || !strings.Contains(body, `"schema_version": 1`) {
		t.Fatalf("report endpoint: %d %s", code, body[:min(len(body), 120)])
	}
	if code, body := get(t, srv.URL+"/"); code != 200 || !strings.Contains(body, "loadforge") {
		t.Fatalf("index endpoint: %d", code)
	}
	if code, body := get(t, srv.URL+"/api/timeline.csv"); code != 200 || !strings.Contains(body, "target_rate_rps") {
		t.Fatalf("csv endpoint: %d", code)
	}
	if code, body := get(t, srv.URL+"/assets/app.js"); code != 200 || !strings.Contains(body, "renderVerdict") {
		t.Fatalf("app.js endpoint: %d", code)
	}
	if code, _ := get(t, srv.URL+"/assets/uplot.js"); code != 200 {
		t.Fatalf("uplot endpoint: %d", code)
	}
}
