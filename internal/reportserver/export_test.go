package reportserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func TestExportCSVCreatesDirectory(t *testing.T) {
	report := &model.Report{
		Meta: model.Meta{
			Tool: "loadforge", Version: "test", Target: "http://x", Method: "GET",
			Mode: "constant", StartedAt: time.Now(), Duration: 1000,
		},
		Summary:  model.Summary{Requests: 10, Rate: 10, SuccessRate: 1},
		Verdict:  model.Verdict{Status: model.VerdictPass, Title: "ok"},
		Timeline: []model.TimelinePoint{{T: 0, Requests: 10, Rate: 10, TargetRate: 10}},
	}

	dir := filepath.Join(t.TempDir(), "out")
	summary, timeline, err := ExportCSV(report, dir)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if summary != filepath.Join(dir, "summary.csv") || timeline != filepath.Join(dir, "timeline.csv") {
		t.Fatalf("paths = %q, %q", summary, timeline)
	}
	data, err := os.ReadFile(summary)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	for _, want := range []string{"section,metric,value", "summary,requests,10", "timings,samples"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("summary missing %q", want)
		}
	}
	if _, err := os.Stat(timeline); err != nil {
		t.Fatalf("timeline: %v", err)
	}
}

func TestExportHTMLIsSelfContained(t *testing.T) {
	report := &model.Report{
		SchemaVersion: model.SchemaVersion,
		Meta: model.Meta{
			Tool:      "loadforge",
			Version:   "test",
			Target:    "http://127.0.0.1:9099/api/data",
			Method:    "GET",
			Mode:      "constant",
			StartedAt: time.Now(),
		},
		Summary: model.Summary{Requests: 10, Rate: 10, SuccessRate: 1},
		Verdict: model.Verdict{Status: model.VerdictPass, Title: "Carga sostenida"},
		Timeline: []model.TimelinePoint{
			{T: 0, Requests: 5, Rate: 5, TargetRate: 5, P95: 10, ClientCPUPercent: 2.5},
			{T: 1, Requests: 5, Rate: 5, TargetRate: 5, P95: 10, ClientCPUPercent: 3.1},
		},
		Environment: model.Environment{OS: "linux", Arch: "amd64", CPUs: 4},
	}

	dir := t.TempDir()
	path, err := ExportHTML(report, dir)
	if err != nil {
		t.Fatalf("ExportHTML: %v", err)
	}
	if filepath.Ext(path) != ".html" {
		t.Fatalf("path = %q, want an .html file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	html := string(data)
	for _, want := range []string{
		"<!doctype html>",
		"<style>",
		"window.__LOADFORGE_REPORT__=",
		"http://127.0.0.1:9099/api/data",
		"client_cpu_percent",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("export is missing %q", want)
		}
	}
	for _, unwanted := range []string{
		`href="/assets/app.css"`,
		`src="/assets/app.js"`,
		`src="/assets/uplot.js"`,
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("export still references %q", unwanted)
		}
	}
}
