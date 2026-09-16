// Package access implements device-side MQTT ingestion for IoLink.
//
// Responsibilities: embedded MQTT broker, device triple authentication,
// topic ACL, payload validation against the wire contract, online/offline
// tracking, and publishing normalized events to core via event.Handler.
//
// This module NEVER imports iolink-core or iolink-appapi — it depends only
// on git.hyhy.fun/rsplab/git.hyhy.fun/rsplab/iolink/internal/domain.
package access

import (
	"context"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"

	iolinkcontracts "git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
)

// Config for the access module.
type Config struct {
	MQTTAddr           string        // e.g. ":1883"
	ReportInterval     time.Duration // expected device reporting period (offline watchdog)
	OfflineGraceFactor int           // offline after N × interval without message
}

// Server = embedded MQTT broker + event normalization.
type Server struct {
	cfg          Config
	handler      iolinkcontracts.Handler
	auth         Authenticator
	log          *slog.Logger
	broker       *mqtt.Server
	connectCh    chan string
	disconnectCh chan string

	mu      sync.Mutex
	lastSee map[string]time.Time
}

// New builds the access server: embedded broker on cfg.MQTTAddr, events
// delivered to `handler` (core), device triples verified by `auth`.
// auth == nil selects NoopAuth (DEV ONLY).
func New(cfg Config, handler iolinkcontracts.Handler, auth Authenticator, log *slog.Logger) *Server {
	if auth == nil {
		log.Warn("NoopAuth in use — DEV ONLY, any device can connect")
		auth = NoopAuth{}
	}
	s := &Server{
		cfg:          cfg,
		handler:      handler,
		auth:         auth,
		log:          log,
		connectCh:    make(chan string, 64),
		disconnectCh: make(chan string, 64),
		lastSee:      make(map[string]time.Time),
	}
	s.broker = mqtt.New(&mqtt.Options{InlineClient: true})
	return s
}

// Serve starts the embedded MQTT broker (blocking). Drive the offline
// watchdog via Run(ctx) in a separate goroutine.
func (s *Server) Serve() error {
	if err := s.broker.AddHook(newBrokerHook(s), nil); err != nil {
		return err
	}
	tcp := listeners.NewTCP(listeners.Config{ID: "iolink-mqtt", Address: s.cfg.MQTTAddr})
	if err := s.broker.AddListener(tcp); err != nil {
		return err
	}
	s.log.Info("mqtt broker listening", "addr", s.cfg.MQTTAddr)
	return s.broker.Serve()
}

// Close shuts the broker down (graceful shutdown from main).
func (s *Server) Close() error { return s.broker.Close() }

// HandleReport processes one properties payload (called by the broker hook;
// also directly usable by tests/simulators).
func (s *Server) HandleReport(deviceNo string, r wire.Report) error {
	e := iolinkcontracts.Event{
		Kind:       iolinkcontracts.KindProperties,
		DeviceNo:   deviceNo,
		Ts:         time.Now().UTC(),
		Properties: make(map[string]float64, 6),
	}
	if r.Temperature != nil {
		e.Properties["temperature"] = *r.Temperature
	}
	if r.DO != nil {
		e.Properties["dissolved_oxygen"] = *r.DO
	}
	if r.PH != nil {
		e.Properties["ph"] = *r.PH
	}
	if r.Turbidity != nil {
		e.Properties["turbidity"] = *r.Turbidity
	}
	if r.Salinity != nil {
		e.Properties["salinity"] = *r.Salinity
	}
	if r.Battery != nil {
		e.Properties["battery"] = *r.Battery
	}
	if r.Signal != nil {
		e.Properties["signal"] = float64(*r.Signal)
	}
	s.touch(deviceNo)
	return s.handler.HandleEvent(e)
}

// Run drives connect/disconnect fan-out + the offline watchdog until ctx is
// cancelled.
func (s *Server) Run(ctx context.Context) {
	t := time.NewTicker(s.cfg.ReportInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case no := <-s.connectCh:
			s.touch(no)
			if err := s.statusEvent(no, true); err != nil {
				s.log.Error("online event", "device", no, "err", err)
			}
		case no := <-s.disconnectCh:
			if err := s.statusEvent(no, false); err != nil {
				s.log.Error("offline event", "device", no, "err", err)
			}
		case <-t.C:
			cutoff := time.Now().Add(-s.cfg.ReportInterval * time.Duration(s.cfg.OfflineGraceFactor))
			for no, seen := range s.snapshot() {
				if seen.Before(cutoff) {
					if err := s.statusEvent(no, false); err != nil {
						s.log.Error("watchdog offline", "device", no, "err", err)
					}
				}
			}
		}
	}
}

func (s *Server) statusEvent(no string, online bool) error {
	return s.handler.HandleEvent(iolinkcontracts.Event{
		Kind: iolinkcontracts.KindStatusChange, DeviceNo: no,
		Ts: time.Now().UTC(), Online: online,
	})
}

func (s *Server) touch(no string) {
	s.mu.Lock()
	s.lastSee[no] = time.Now()
	s.mu.Unlock()
}

func (s *Server) snapshot() map[string]time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]time.Time, len(s.lastSee))
	for k, v := range s.lastSee {
		out[k] = v
	}
	return out
}
