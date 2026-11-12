package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CheckTarget validates the target without generating load. It is used by
// `loadforge run --dry-run`.
func CheckTarget(ctx context.Context, cfg Config) error {
	cfg.Preflight = true
	var oauth *oauthSource
	if cfg.OAuth.TokenURL != "" {
		oauth = newOAuthSource(cfg.OAuth)
		if _, err := oauth.tokenFor(ctx); err != nil {
			return err
		}
	}
	return preflight(ctx, cfg, newHTTPClient(cfg, oauth))
}

// preflight probes the endpoint once before generating load. It catches the
// most common configuration mistakes (wrong route, bad credentials, service
// down) without polluting the measured results.
//
// It is intentionally limited to safe methods: sending a real POST/PUT/PATCH/
// DELETE before the test could create or modify data twice.
func preflight(ctx context.Context, cfg Config, client *http.Client) error {
	if !cfg.Preflight || cfg.Scenario != nil {
		return nil
	}
	switch cfg.Method {
	case http.MethodGet, http.MethodHead:
	default:
		return nil
	}

	timeout := cfg.Timeout
	if timeout <= 0 || timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, cfg.Method, cfg.URL, nil)
	if err != nil {
		return nil
	}
	req.Header = cfg.Headers.Clone()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("la comprobación previa no pudo conectar con %s: %w", sanitizeURL(cfg.URL), err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf(
			"la comprobación previa recibió %d: la API key o el esquema de autenticación no son válidos. Revisa --api-key-header y --api-key-prefix",
			resp.StatusCode,
		)
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return fmt.Errorf(
			"la comprobación previa recibió %d: el endpoint no existe o el método %s no está permitido. Revisa la URL: %s",
			resp.StatusCode, cfg.Method, sanitizeURL(cfg.URL),
		)
	}

	// 5xx and 429 may be transient or caused by load itself; let the test run
	// and measure them instead of refusing to start.
	return nil
}
