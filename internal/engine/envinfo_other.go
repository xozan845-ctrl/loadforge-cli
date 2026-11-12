//go:build !linux && !android && !darwin && !windows

package engine

// readEnvSample is a no-op on platforms without a resource collector. The
// report still carries the portable runtime fields.
func readEnvSample() envSample { return envSample{} }
