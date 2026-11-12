package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOAuthSourceAndTransport(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("form: %v", err)
		}
		if r.Form.Get("grant_type") != "client_credentials" ||
			r.Form.Get("client_id") != "id" ||
			r.Form.Get("client_secret") != "secret" {
			t.Errorf("unexpected token request: %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       map[string]any{"token": "abc123"},
			"expires_in": 3600,
		})
	}))
	defer tokenSrv.Close()

	var gotAuth string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	cfg := Config{
		URL:    apiSrv.URL,
		Method: http.MethodGet,
		OAuth: OAuth2Config{
			TokenURL:     tokenSrv.URL,
			ClientID:     "id",
			ClientSecret: "secret",
			TokenField:   "data.token",
		},
		Timeout: 0,
	}
	src := newOAuthSource(cfg.OAuth)
	if _, err := src.tokenFor(context.Background()); err != nil {
		t.Fatalf("token: %v", err)
	}
	client := newHTTPClient(cfg, src)
	resp, err := client.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if gotAuth != "Bearer abc123" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}
