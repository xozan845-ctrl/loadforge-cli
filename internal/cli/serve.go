package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xozan845-ctrl/loadforge-cli/internal/model"
	"github.com/xozan845-ctrl/loadforge-cli/internal/reportserver"
)

func newServeCmd() *cobra.Command {
	var (
		host   string
		port   int
		noOpen bool
	)
	cmd := &cobra.Command{
		Use:   "serve <reporte.json>",
		Short: "Sirve un reporte guardado en localhost",
		Long: `Abre un reporte JSON guardado con --save en el servidor web local,
sin repetir la prueba de carga.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("no se pudo leer el reporte: %w", err)
			}
			var report model.Report
			if err := json.Unmarshal(data, &report); err != nil {
				return fmt.Errorf("JSON inválido: %w", err)
			}
			if report.Meta.Tool != "loadforge" {
				return fmt.Errorf("el archivo no parece un reporte de LoadForge")
			}

			srv, err := reportserver.Serve(&report, host, port, !noOpen)
			if err != nil {
				return err
			}
			defer srv.Close()

			printReportURL(srv.URL)
			waitForInterrupt()
			fmt.Println()
			fmt.Println("  " + mutedStyle.Render("Servidor cerrado."))
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&host, "host", "127.0.0.1", "Interfaz donde servir el reporte")
	f.IntVar(&port, "port", 0, "Puerto (0 = aleatorio)")
	f.BoolVar(&noOpen, "no-open", false, "No abrir el navegador automáticamente")
	return cmd
}
