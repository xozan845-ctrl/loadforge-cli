package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var templateVar = regexp.MustCompile(`\{\{\.([A-Za-z0-9_]+)\}\}`)

// loadDataFile reads a CSV or JSON array into one map per request.
func loadDataFile(path string) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer --data-file: %w", err)
	}

	if strings.EqualFold(filepath.Ext(path), ".csv") {
		return parseCSVData(data)
	}
	return parseJSONData(data)
}

func parseCSVData(data []byte) ([]map[string]string, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV inválido: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("el CSV debe tener cabecera y al menos una fila")
	}
	headers := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) != len(headers) {
			return nil, fmt.Errorf("fila %d: %d columnas, se esperaban %d", i+2, len(rec), len(headers))
		}
		row := make(map[string]string, len(headers))
		for j, h := range headers {
			row[strings.TrimSpace(h)] = rec[j]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseJSONData(data []byte) ([]map[string]string, error) {
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("JSON inválido (se espera un array de objetos): %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("el JSON no contiene filas")
	}
	rows := make([]map[string]string, 0, len(raw))
	for _, item := range raw {
		row := make(map[string]string, len(item))
		for k, v := range item {
			switch value := v.(type) {
			case string:
				row[k] = value
			case float64:
				row[k] = strings.TrimSuffix(fmt.Sprintf("%f", value), ".000000")
			case bool:
				row[k] = fmt.Sprintf("%t", value)
			case nil:
				row[k] = ""
			default:
				encoded, _ := json.Marshal(value)
				row[k] = string(encoded)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func validateTemplate(tmpl string, rows []map[string]string) error {
	if tmpl == "" {
		return nil
	}
	keys := map[string]bool{"Iteration": true, "RandomID": true}
	for _, row := range rows {
		for k := range row {
			keys[k] = true
		}
	}
	for _, match := range templateVar.FindAllStringSubmatch(tmpl, -1) {
		if !keys[match[1]] {
			return fmt.Errorf("la plantilla usa {{.%s}} pero esa columna no existe en --data-file", match[1])
		}
	}
	return nil
}
