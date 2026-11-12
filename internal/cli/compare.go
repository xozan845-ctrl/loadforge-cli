package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xozan845-ctrl/loadforge-cli/internal/compare"
	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func newCompareCmd() *cobra.Command {
	var (
		jsonOut          bool
		failOnRegression bool
	)
	cmd := &cobra.Command{
		Use:   "compare <baseline.json> <actual.json>",
		Short: "Compara dos reportes y detecta regresiones",
		Long: `Compara una ejecución actual contra un reporte base guardado con --save.
Marca regresiones de latencia, errores y throughput.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := loadReport(args[0])
			if err != nil {
				return fmt.Errorf("baseline: %w", err)
			}
			current, err := loadReport(args[1])
			if err != nil {
				return fmt.Errorf("actual: %w", err)
			}

			cmp := compare.Build(base, current)
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(cmp); err != nil {
					return err
				}
			} else {
				printComparisonTable(&cmp, base, current)
			}

			if failOnRegression && len(cmp.Regressions) > 0 {
				return fmt.Errorf("se detectaron %d regresiones", len(cmp.Regressions))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Imprimir la comparación en JSON")
	cmd.Flags().BoolVar(&failOnRegression, "fail-on-regression", false, "Salir con código 1 si hay regresiones")
	return cmd
}

func printComparisonTable(cmp *model.Comparison, base, current *model.Report) {
	fmt.Println()
	fmt.Println("  " + labelStyle.Render("Comparación con baseline"))
	fmt.Printf("  %s %s → %s\n", mutedStyle.Render("Objetivo:"),
		base.Meta.Target, current.Meta.Target)
	if cmp.TargetChanged {
		fmt.Println("  " + warnStyle.Render("⚠") + " el objetivo cambió; la comparación puede no ser significativa")
	}

	fmt.Println()
	header := fmt.Sprintf("  %-16s %-12s %-12s %-10s", "Métrica", "Base", "Actual", "Cambio")
	fmt.Println(mutedStyle.Render(header))
	for _, m := range cmp.Metrics {
		mark := " "
		if m.Regressed {
			mark = warnStyle.Render("⚠")
		} else if m.Improved {
			mark = okStyle.Render("✓")
		}
		delta := compareDelta(m)
		fmt.Printf("  %-16s %-12s %-12s %-10s %s\n",
			m.Label, compareValue(m, m.Baseline), compareValue(m, m.Current), delta, mark)
	}

	fmt.Println()
	switch {
	case len(cmp.Regressions) > 0:
		fmt.Printf("  %s %d regresiones: %s\n", warnStyle.Render("⚠"),
			len(cmp.Regressions), strings.Join(cmp.Regressions, ", "))
	case len(cmp.Improvements) > 0:
		fmt.Printf("  %s mejoras: %s\n", okStyle.Render("✓"), strings.Join(cmp.Improvements, ", "))
	default:
		fmt.Println("  " + mutedStyle.Render("Sin cambios significativos."))
	}
	fmt.Println()
}

func compareValue(m model.ComparisonMetric, v float64) string {
	switch m.Unit {
	case "ms":
		return formatMs(v)
	case "%":
		return fmt.Sprintf("%.2f%%", v*100)
	default:
		return fmt.Sprintf("%.1f %s", v, m.Unit)
	}
}

func compareDelta(m model.ComparisonMetric) string {
	if m.Unit == "%" {
		return fmt.Sprintf("%+.2f pp", m.DeltaPoints)
	}
	return fmt.Sprintf("%+.1f%%", m.DeltaPercent)
}
