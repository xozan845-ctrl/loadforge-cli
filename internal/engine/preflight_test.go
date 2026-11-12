package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func preflightConfig(url, method string) Config {
	return Config{
		URL:       url,
		Method:    method,
		Headers:   http.Header{},
		Timeout:   2 * time.Second,
		Preflight: true,
	}
}

func runPreflight(cfg Config) error {
	return preflight(context.Background(), cfg, newHTTPClient(cfg, nil))
}

func TestPreflightAcceptsHealthyEndpoint(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := runPreflight(preflightConfig(srv.URL, http.MethodGet)); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
}

func TestPreflightRejectsMissingRoute(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	err := runPreflight(preflightConfig(srv.URL, http.MethodGet))
	if err == nil {
		t.Fatal("expected an error for 404")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreflightRejectsBadCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := runPreflight(preflightConfig(srv.URL, http.MethodGet))
	if err == nil {
		t.Fatal("expected an error for 401")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreflightLetsServerErrorsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := runPreflight(preflightConfig(srv.URL, http.MethodGet)); err != nil {
		t.Fatalf("5xx should not abort the test: %v", err)
	}
}

func TestPreflightSkipsUnsafeMethods(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := runPreflight(preflightConfig(srv.URL, http.MethodPost)); err != nil {
		t.Fatalf("POST should skip preflight: %v", err)
	}
	if hits != 0 {
		t.Fatalf("hits = %d, want 0 for an unsafe method", hits)
	}
}

func TestPreflightDisabled(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := preflightConfig(srv.URL, http.MethodGet)
	cfg.Preflight = false
	if err := runPreflight(cfg); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if hits != 0 {
		t.Fatalf("hits = %d, want 0 when disabled", hits)
	}
}
