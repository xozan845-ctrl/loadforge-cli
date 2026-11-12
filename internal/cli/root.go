// Package cli wires the commands, the engine and the reporter together.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xozan845-ctrl/loadforge-cli/internal/version"
)

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("✕ ")+err.Error())
		return 1
	}
	return 0
}

// NewRootCmd builds the command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "loadforge",
		Short: "Pruebas de carga para APIs con reporte web local",
		Long: `LoadForge mide cuánto aguanta tu backend.

Genera carga HTTP real, calcula latencias y errores en vivo, estima el punto
de quiebre y sirve un reporte interactivo en localhost. Funciona en Termux,
Linux, macOS y Windows con un único binario.

La API key se puede pasar con --api-key o, mejor, con la variable de entorno
LOADFORGE_API_KEY para que no quede en el historial del shell.`,
		Example: `  # Rampa de 1 a 50 req/s durante 30s con Bearer token
  loadforge run --url https://api.ejemplo.com/health --api-key "$TOKEN"

  # Carga constante de 100 req/s durante 1 minuto
  loadforge run --url https://api.ejemplo.com/v1/items \
    --api-key "$TOKEN" --mode constant --rate 100 --duration 1m

  # POST con cuerpo y header X-API-Key
  loadforge run --url https://api.ejemplo.com/v1/items \
    --api-key "$TOKEN" --api-key-header X-API-Key --api-key-prefix "" \
    --method POST --body '{"name":"test"}'

  # Solo JSON, sin servidor web (CI)
  loadforge run --url https://api.ejemplo.com/health --json --save reporte.json`,
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.AddCommand(newRunCmd())
	return root
}
