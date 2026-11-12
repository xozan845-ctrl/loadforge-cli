// Command mockserver is a small, intentionally imperfect HTTP server used to
// exercise LoadForge locally. It is not part of the shipped CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9099", "dirección de escucha")
	latency := flag.Duration("latency", 15*time.Millisecond, "latencia base")
	jitter := flag.Duration("jitter", 25*time.Millisecond, "jitter aleatorio adicional")
	failRate := flag.Float64("fail", 0.01, "probabilidad de respuesta 500")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		d := *latency
		if *jitter > 0 {
			d += time.Duration(rand.Int63n(int64(*jitter)))
		}
		time.Sleep(d)
		if rand.Float64() < *failRate {
			http.Error(w, `{"error":"fallo simulado"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"latency_ms":%d,"method":%q,"ts":%q}`,
			d.Milliseconds(), r.Method, time.Now().Format(time.RFC3339Nano))
	})
	mux.HandleFunc("/secure", func(w http.ResponseWriter, r *http.Request) {
		authorized := r.Header.Get("Authorization") == "Bearer demo" || r.Header.Get("X-API-Key") == "demo"
		if !authorized {
			http.Error(w, `{"error":"no autorizado"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"auth":"ok"}`))
	})

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("mock escuchando en http://%s\n", *addr)
	fmt.Printf("  GET /api/data   latencia %s + jitter %s, fallos %.1f%%\n", *latency, *jitter, *failRate*100)
	fmt.Printf("  GET /healthz    respuesta inmediata\n")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
