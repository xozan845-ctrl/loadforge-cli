package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
)

func defaultHistoryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".loadforge-history"
	}
	return filepath.Join(home, ".local", "share", "loadforge", "history")
}

func loadHistory(dir string) ([]*model.Report, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var reports []*model.Report
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		report, err := loadReport(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Meta.StartedAt.After(reports[j].Meta.StartedAt)
	})
	return reports, nil
}

func newHistoryCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "history [directorio]",
		Short: "Lista las ejecuciones guardadas en el historial local",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := defaultHistoryDir()
			if len(args) == 1 {
				dir = args[0]
			}
			reports, err := loadHistory(dir)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(reports)
			}
			if len(reports) == 0 {
				fmt.Println("  " + mutedStyle.Render("No hay ejecuciones en "+dir))
				return nil
			}
			fmt.Println()
			fmt.Println("  " + labelStyle.Render("Historial · "+dir))
			fmt.Println()
			fmt.Printf("  %-19s %-28s %-10s %-9s %s\n",
				"Fecha", "Objetivo", "Modo", "P95", "Veredicto")
			for _, report := range reports {
				target := report.Meta.Target
				if len(target) > 28 {
					target = target[:27] + "…"
				}
				fmt.Printf("  %-19s %-28s %-10s %-9s %s\n",
					report.Meta.StartedAt.Local().Format("2006-01-02 15:04:05"),
					target,
					report.Meta.Mode,
					formatMs(report.Latency.P95),
					report.Verdict.Title,
				)
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Imprimir el historial completo en JSON")
	return cmd
}

func historyPoint(report *model.Report) model.HistoryPoint {
	return model.HistoryPoint{
		At:        report.Meta.StartedAt,
		Mode:      report.Meta.Mode,
		P95Ms:     report.Latency.P95,
		Rate:      report.Summary.Rate,
		ErrorRate: report.Summary.ErrorRate,
		Verdict:   report.Verdict.Status,
	}
}

func historyFileName(now time.Time) string {
	return "run-" + now.Format("20060102-150405.000") + ".json"
}
