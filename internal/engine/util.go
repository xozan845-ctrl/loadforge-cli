package engine

import (
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

func httpStatusText(code int) string { return http.StatusText(code) }

func runtimeOS() string      { return runtime.GOOS }
func runtimeArch() string    { return runtime.GOARCH }
func runtimeNumCPU() int     { return runtime.NumCPU() }
func runtimeVersion() string { return runtime.Version() }

// SafeURL returns a version of raw with credentials and secret query values
// redacted. It is exported for terminal/report display.
func SafeURL(raw string) string { return sanitizeURL(raw) }

// sanitizeURL removes credentials and well-known secret query parameters
// before the target is embedded in a report.
func sanitizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
		}
	}
	q := u.Query()
	changed := false
	for key := range q {
		lk := strings.ToLower(key)
		if strings.Contains(lk, "token") || strings.Contains(lk, "secret") ||
			strings.Contains(lk, "password") || strings.Contains(lk, "apikey") ||
			strings.Contains(lk, "api_key") || strings.Contains(lk, "signature") ||
			strings.Contains(lk, "sig") || strings.Contains(lk, "auth") {
			q[key] = []string{"***"}
			changed = true
		}
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	return u.String()
}
