// Package event defines the standardized events that flow access -> core.
//
// In phase 1 these travel over an in-process channel; the struct shapes are
// chosen so a message queue can replace the channel later without touching
// either side.
package event

import "time"

// Kind enumerates event types emitted by access.
type Kind string

const (
	KindProperties   Kind = "properties"   // periodic telemetry report
	KindStatusChange Kind = "status_change" // device online/offline
)

// Event is the single envelope access publishes for every upstream message.
type Event struct {
	Kind      Kind
	DeviceNo  string
	Ts        time.Time
	// Properties payload (Kind == KindProperties): metric name -> value.
	Properties map[string]float64
	// Status payload (Kind == KindStatusChange).
	Online bool
}

// Handler consumes events produced by access. core implements this;
// the wiring in cmd/iolinkd connects the two.
type Handler interface {
	HandleEvent(e Event) error
}