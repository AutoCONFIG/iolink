package operations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpointsReportDependenciesAndDrainState(t *testing.T) {
	h := NewHealth(Checks{Ping: func(context.Context) error { return nil }, Ready: func(context.Context) error { return nil }})
	health := httptest.NewRecorder()
	h.Healthz().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	ready := httptest.NewRecorder()
	h.Readyz().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if health.Code != http.StatusOK || ready.Code != http.StatusOK {
		t.Fatalf("healthy status health=%d ready=%d", health.Code, ready.Code)
	}

	h.SetReady(false)
	ready = httptest.NewRecorder()
	h.Readyz().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	health = httptest.NewRecorder()
	h.Healthz().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if ready.Code != http.StatusServiceUnavailable || health.Code != http.StatusOK {
		t.Fatalf("draining status health=%d ready=%d", health.Code, ready.Code)
	}
}

func TestHealthEndpointsFailClosedOnDependencyErrors(t *testing.T) {
	h := NewHealth(Checks{Ping: func(context.Context) error { return errors.New("down") }, Ready: func(context.Context) error { return errors.New("pending") }})
	for name, handler := range map[string]http.Handler{"health": h.Healthz(), "ready": h.Readyz()} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d", name, recorder.Code)
		}
	}
}
