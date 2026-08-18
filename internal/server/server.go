// Package server — HTTP :8080. Paths mirror Spring Actuator so
// k8s probes and Prometheus scrape config remain unchanged when moving to Go.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// pingTimeout — upper bound on DB ping timeout inside /actuator/health
// and /actuator/health/readiness, so a stuck DB does not hang the probe itself.
const pingTimeout = 5 * time.Second

// New assembles an actuator-like HTTP server. ping is an optional
// readiness check (usually pool.Ping); if provided and fails,
// /actuator/health and /actuator/health/readiness return 503 DOWN — like
// Spring's aggregate health, which goes down when the DB is unreachable
// (thus removing the pod from k8s rotation). /actuator/health/liveness
// is always UP: the process is alive regardless of DB state.
func New(addr string, ping func(context.Context) error) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/actuator/health", healthHandler(ping))
	mux.HandleFunc("/actuator/health/liveness", healthHandler(nil))
	mux.HandleFunc("/actuator/health/readiness", healthHandler(ping))
	mux.Handle("/actuator/prometheus", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
}

func healthHandler(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ping != nil {
			ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
			defer cancel()
			if err := ping(ctx); err != nil {
				writeStatus(w, http.StatusServiceUnavailable, "DOWN")
				return
			}
		}
		writeStatus(w, http.StatusOK, "UP")
	}
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"status":"` + status + `"}`))
}
