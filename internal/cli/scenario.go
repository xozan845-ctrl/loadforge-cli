package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xozan845-ctrl/loadforge-cli/internal/engine"
)

func loadScenario(path string) (*engine.Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer --scenario: %w", err)
	}
	var sc engine.Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("escenario JSON inválido: %w", err)
	}
	if len(sc.Steps) == 0 {
		return nil, fmt.Errorf("el escenario no tiene pasos")
	}
	for i := range sc.Steps {
		if sc.Steps[i].Method == "" {
			sc.Steps[i].Method = "GET"
		}
		if sc.Steps[i].Name == "" {
			sc.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}
	}
	return &sc, nil
}
