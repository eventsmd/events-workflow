// Package server — HTTP :8080. Пути повторяют Spring Actuator, чтобы
// k8s-пробы и scrape-конфиг Prometheus не менялись при переезде на Go.
package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New(addr string) *http.Server {
	mux := http.NewServeMux()
	up := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"UP"}`))
	}
	mux.HandleFunc("/actuator/health", up)
	mux.HandleFunc("/actuator/health/liveness", up)
	mux.HandleFunc("/actuator/health/readiness", up)
	mux.Handle("/actuator/prometheus", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
}
