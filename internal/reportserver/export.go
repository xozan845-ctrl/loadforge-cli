package reportserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// ExportHTML writes a single self-contained HTML file: markup, styles, uPlot,
// app code and the report data are all inlined, so the file works offline and
// can be shared as a CI artifact or attached to a ticket.
func ExportHTML(report *model.Report, path string) (string, error) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		path = filepath.Join(path, "loadforge-"+time.Now().Format("20060102-150405")+".html")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	index, err := webFS.ReadFile("web/index.html")
	if err != nil {
		return "", err
	}
	appCSS, err := webFS.ReadFile("web/style.css")
	if err != nil {
		return "", err
	}
	appJS, err := webFS.ReadFile("web/app.js")
	if err != nil {
		return "", err
	}
	plotCSS, err := assetsFS.ReadFile("assets/uPlot.min.css")
	if err != nil {
		return "", err
	}
	plotJS, err := assetsFS.ReadFile("assets/uPlot.iife.min.js")
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(report)
	if err != nil {
		return "", err
	}

	html := string(index)
	replacements := []struct {
		old string
		new string
	}{
		{
			`<link rel="stylesheet" href="/assets/uplot.css">`,
			"<style>\n" + string(plotCSS) + "\n</style>",
		},
		{
			`<link rel="stylesheet" href="/assets/app.css">`,
			"<style>\n" + string(appCSS) + "\n</style>",
		},
		{
			`<script src="/assets/uplot.js"></script>`,
			"<script>\n" + string(plotJS) + "\n</script>",
		},
		{
			`<script src="/assets/app.js"></script>`,
			"<script>window.__LOADFORGE_REPORT__=" + string(data) + ";</script>\n" +
				"<script>\n" + string(appJS) + "\n</script>",
		},
	}
	for _, r := range replacements {
		if !strings.Contains(html, r.old) {
			return "", fmt.Errorf("plantilla HTML inesperada: no se encontró %q", r.old)
		}
		html = strings.Replace(html, r.old, r.new, 1)
	}

	if err := os.WriteFile(path, []byte(html), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
