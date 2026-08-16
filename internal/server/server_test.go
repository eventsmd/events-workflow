package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActuatorEndpoints(t *testing.T) {
	s := New(":0")
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
