package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OAuth2Config describes a client-credentials token source.
type OAuth2Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
	TokenField   string // dot path, e.g. "access_token" or "data.token"
	AuthStyle    string // "body" (default) or "basic"
}

// oauthSource keeps a bearer token fresh and injects it into every request.
type oauthSource struct {
	cfg OAuth2Config

	mu      sync.Mutex
	token   string
	expires time.Time
}

func newOAuthSource(cfg OAuth2Config) *oauthSource {
	if cfg.TokenField == "" {
		cfg.TokenField = "access_token"
	}
	return &oauthSource{cfg: cfg}
}

func (o *oauthSource) tokenFor(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.token != "" && time.Until(o.expires) > 30*time.Second {
		return o.token, nil
	}
	return o.refreshLocked(ctx)
}

func (o *oauthSource) refreshLocked(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if o.cfg.Scope != "" {
		form.Set("scope", o.cfg.Scope)
	}
	if o.cfg.AuthStyle != "basic" {
		form.Set("client_id", o.cfg.ClientID)
		form.Set("client_secret", o.cfg.ClientSecret)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if o.cfg.AuthStyle == "basic" {
		req.SetBasicAuth(o.cfg.ClientID, o.cfg.ClientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token OAuth2: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token OAuth2: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("token OAuth2: respuesta JSON inválida: %w", err)
	}
	token := nestedString(payload, o.cfg.TokenField)
	if token == "" {
		return "", fmt.Errorf("token OAuth2: no se encontró %q en la respuesta", o.cfg.TokenField)
	}
	expiresIn := int64(3600)
	if v := nestedValue(payload, "expires_in"); v != nil {
		switch n := v.(type) {
		case float64:
			expiresIn = int64(n)
		case string:
			if parsed, err := strconv.ParseInt(n, 10, 64); err == nil {
				expiresIn = parsed
			}
		}
	}
	o.token = token
	o.expires = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return token, nil
}

func nestedValue(payload map[string]any, path string) any {
	if path == "" {
		return nil
	}
	var current any = payload
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

func nestedString(payload map[string]any, path string) string {
	if s, ok := nestedValue(payload, path).(string); ok {
		return s
	}
	return ""
}

// oauthTransport injects a fresh bearer token into each request.
type oauthTransport struct {
	base   http.RoundTripper
	source *oauthSource
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.source.tokenFor(req.Context())
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(clone)
}
