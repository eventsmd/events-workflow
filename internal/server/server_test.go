package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActuatorEndpoints_HealthyDB(t *testing.T) {
	s := New(":0", func(context.Context) error { return nil })
	for _, path := range []string{
		"/actuator/health", "/actuator/health/liveness", "/actuator/health/readiness",
	} {
		rec := httptest.NewRecorder()
		s.Handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"UP"`) {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	s.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/actuator/prometheus", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("prometheus: %d", rec.Code)
	}
}

func TestActuatorEndpoints_NilPing_AlwaysUP(t *testing.T) {
	s := New(":0", nil)
	for _, path := range []string{
		"/actuator/health", "/actuator/health/liveness", "/actuator/health/readiness",
	} {
		rec := httptest.NewRecorder()
		s.Handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"UP"`) {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

// TestActuatorEndpoints_DBDown — parity with Spring's aggregate health,
// which goes DOWN when the DB is unreachable so k8s pulls the pod out of
// rotation. Liveness must stay UP unconditionally: the process itself is
// still alive.
func TestActuatorEndpoints_DBDown(t *testing.T) {
	s := New(":0", func(context.Context) error { return errors.New("connection refused") })

	for _, path := range []string{"/actuator/health", "/actuator/health/readiness"} {
		rec := httptest.NewRecorder()
		s.Handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 503 || !strings.Contains(rec.Body.String(), `"status":"DOWN"`) {
			t.Fatalf("%s: want 503 DOWN, got %d %s", path, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	s.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/actuator/health/liveness", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"UP"`) {
		t.Fatalf("liveness must stay UP when DB is down: %d %s", rec.Code, rec.Body.String())
	}
}
