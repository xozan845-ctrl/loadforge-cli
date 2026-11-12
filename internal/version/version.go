// Package version holds the build version, overridable with:
//
//	go build -ldflags "-X loadforge/internal/version.Version=1.2.3"
package version

// Version is the current LoadForge version.
var Version = "0.1.0"

// UserAgent is sent with every generated request.
func UserAgent() string { return "github.com/xozan845-ctrl/loadforge-cli/" + Version }
