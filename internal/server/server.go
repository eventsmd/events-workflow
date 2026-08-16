// Package server — HTTP :8080. Пути повторяют Spring Actuator, чтобы
// k8s-пробы и scrape-конфиг Prometheus не менялись при переезде на Go.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// pingTimeout — верхняя граница ожидания DB-пинга внутри /actuator/health
// и /actuator/health/readiness, чтобы застрявшая БД не вешала саму пробу.
const pingTimeout = 5 * time.Second

// New собирает actuator-подобный HTTP-сервер. ping — необязательная
// проверка готовности (обычно pool.Ping); если она задана и падает,
// /actuator/health и /actuator/health/readiness отдают 503 DOWN — как
// Spring's aggregate health, который уводит под down при недоступной БД
// (и тем самым убирает под из ротации k8s). /actuator/health/liveness
// всегда UP: процесс жив независимо от состояния БД.
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
