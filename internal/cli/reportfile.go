package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

// loadReport reads and validates a report saved with --save.
func loadReport(path string) (*model.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el reporte: %w", err)
	}
	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("JSON inválido: %w", err)
	}
	if report.Meta.Tool != "loadforge" {
		return nil, fmt.Errorf("el archivo no parece un reporte de LoadForge")
	}
	return &report, nil
}
