package operations

import (
	"context"
	"net/http"
	"sync/atomic"
)

type Checks struct {
	Ping  func(context.Context) error
	Ready func(context.Context) error
}

type Health struct {
	checks Checks
	ready  atomic.Bool
}

func NewHealth(checks Checks) *Health {
	h := &Health{checks: checks}
	h.ready.Store(true)
	return h
}

func (h *Health) SetReady(ready bool) { h.ready.Store(ready) }

func (h *Health) Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.checks.Ping != nil {
			if err := h.checks.Ping(r.Context()); err != nil {
				http.Error(w, "db unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
}

func (h *Health) Readyz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.ready.Load() {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		if h.checks.Ready != nil {
			if err := h.checks.Ready(r.Context()); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ready\n"))
	})
}
