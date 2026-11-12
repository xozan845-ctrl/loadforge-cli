# Terceros

LoadForge usa y distribuye los siguientes proyectos de código abierto. Gracias
a sus autores y mantenedores.

| Proyecto | Uso | Licencia |
|---|---|---|
| [Vegeta](https://github.com/tsenart/vegeta) | Motor de generación de carga y modelo de resultados | MIT |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | TUI en vivo | MIT |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | Estilos de terminal | MIT |
| [Cobra](https://github.com/spf13/cobra) | CLI y flags | Apache-2.0 |
| [HdrHistogram-go](https://github.com/HdrHistogram/hdrhistogram-go) | Percentiles de latencia | MIT |
| [uPlot](https://github.com/leeoniya/uPlot) | Gráficas del reporte web (embebido) | MIT |
| [Go x/term](https://pkg.go.dev/golang.org/x/term) | Detección de terminal | BSD-3-Clause |
| [Go](https://go.dev) | Runtime y toolchain | BSD-3-Clause |

Los archivos de uPlot incluidos (`internal/reportserver/assets/`) conservan su
licencia original. El resto de dependencias se resuelven vía `go.mod`.
