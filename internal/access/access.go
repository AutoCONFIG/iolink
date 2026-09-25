// Package access authenticates MQTT devices and normalizes incoming events.
package access

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type Config struct {
	MQTTAddr           string
	ReportInterval     time.Duration
	OfflineGraceFactor int
}
type session struct {
	mu       sync.Mutex
	interval time.Duration
	checking bool
	client   *mqtt.Client
	lastSeen time.Time
	online   bool
	revision int64
}
type Server struct {
	cfg      Config
	handler  event.Handler
	auth     Authenticator
	log      *slog.Logger
	broker   *mqtt.Server
	mu       sync.Mutex
	sessions map[string]*session
	now      func() time.Time
	workers  sync.WaitGroup
	slots    chan struct{}
}

func New(cfg Config, handler event.Handler, auth Authenticator, log *slog.Logger) *Server {
	if cfg.ReportInterval <= 0 {
		cfg.ReportInterval = time.Minute
	}
	if cfg.OfflineGraceFactor <= 0 {
		cfg.OfflineGraceFactor = 3
	}
	if auth == nil {
		auth = denyAuth{}
	}
	s := &Server{cfg: cfg, handler: handler, auth: auth, log: log, sessions: map[string]*session{}, now: time.Now, slots: make(chan struct{}, 16)}
	caps := mqtt.NewDefaultServerCapabilities()
	// Bound allocations before decoding, allowing MQTT headers beyond 64KiB payload.
	caps.MaximumPacketSize = 65536 + 1024
	s.broker = mqtt.New(&mqtt.Options{InlineClient: true, Capabilities: caps, Logger: log})
	return s
}

type denyAuth struct{}

func (denyAuth) Authenticate(string, string) bool { return false }
func (s *Server) Serve() error {
	if err := s.broker.AddHook(newBrokerHook(s), nil); err != nil {
		return err
	}
	if err := s.broker.AddListener(listeners.NewTCP(listeners.Config{ID: "iolink-mqtt", Address: s.cfg.MQTTAddr})); err != nil {
		return err
	}
	return s.broker.Serve()
}
func (s *Server) Close() error { return s.broker.Close() }

var fieldRanges = map[string][2]float64{"temperature": {0, 50}, "dissolved_oxygen": {0, 20}, "ph": {0, 14}, "turbidity": {0, 1000}, "salinity": {0, 50}, "battery": {0, 100}, "signal": {-120, 0}}

func (s *Server) HandleReport(no string, r wire.Report) error {
	e := event.Event{Kind: event.KindProperties, DeviceNo: no, Ts: s.now().UTC(), Properties: map[string]float64{}}
	if r.MessageID != nil {
		if !utf8.ValidString(*r.MessageID) || utf8.RuneCountInString(*r.MessageID) < 1 || utf8.RuneCountInString(*r.MessageID) > 128 {
			return errors.New("invalid message_id")
		}
		e.MessageID = *r.MessageID
	}
	for k, p := range map[string]*float64{"temperature": r.Temperature, "dissolved_oxygen": r.DO, "ph": r.PH, "turbidity": r.Turbidity, "salinity": r.Salinity, "battery": r.Battery} {
		if p != nil {
			e.Properties[k] = *p
		}
	}
	if r.Signal != nil {
		e.Properties["signal"] = float64(*r.Signal)
	}
	for k, v := range e.Properties {
		rg := fieldRanges[k]
		if math.IsNaN(v) || math.IsInf(v, 0) || v < rg[0] || v > rg[1] {
			delete(e.Properties, k)
			MetricRejected.WithLabelValues("range").Inc()
			s.log.Warn("field out of range", "device", no, "field", k)
		}
	}
	// Valid authenticated packets are heartbeats even when they contain no samples.
	if ss := s.lookup(no, false); ss != nil {
		ss.mu.Lock()
		if ss.client != nil {
			ss.lastSeen = e.Ts
			if err := s.transition(no, ss, true); err != nil {
				ss.mu.Unlock()
				return err
			}
		}
		ss.mu.Unlock()
	}
	if len(e.Properties) == 0 {
		return nil
	}
	if err := s.handler.HandleEvent(e); err != nil {
		MetricRejected.WithLabelValues("persistence").Inc()
		return err
	}
	return nil
}

// All session transitions are synchronous and serialized: no lossy status queue.
func (s *Server) transition(no string, ss *session, online bool) error {
	if ss.online == online && !online {
		return nil
	}
	if err := s.handler.HandleEvent(event.Event{Kind: event.KindStatusChange, DeviceNo: no, Ts: s.now().UTC(), Online: online}); err != nil {
		return err
	}
	ss.online = online
	return nil
}

// Registry lock never spans I/O. Each device serializes its own transitions.
func (s *Server) lookup(no string, create bool) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.sessions[no]
	if ss == nil && create {
		ss = &session{interval: s.cfg.ReportInterval}
		s.sessions[no] = ss
	}
	return ss
}
func (s *Server) connected(cl *mqtt.Client) {
	no := string(cl.Properties.Username)
	interval := s.cfg.ReportInterval
	if p, ok := s.auth.(interface{ ReportInterval(string) time.Duration }); ok {
		if d := p.ReportInterval(no); d > 0 {
			interval = d
		}
	}
	ss := s.lookup(no, true)
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if cl.IsTakenOver() {
		return
	}
	ss.client = cl
	ss.interval = interval
	if p, ok := s.auth.(interface{ DeviceRevision(string) int64 }); ok {
		ss.revision = p.DeviceRevision(no)
	}
	ss.lastSeen = s.now()
	if err := s.transition(no, ss, true); err != nil {
		s.log.Error("online persistence", "device", no, "err", err)
	}
}
func (s *Server) disconnected(cl *mqtt.Client) {
	no := string(cl.Properties.Username)
	ss := s.lookup(no, false)
	if ss == nil {
		return
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.client != cl || cl.IsTakenOver() {
		return
	}
	ss.client = nil
	if err := s.transition(no, ss, false); err != nil {
		s.log.Error("offline persistence", "device", no, "err", err)
	}
}
func (s *Server) isCurrent(cl *mqtt.Client) bool {
	ss := s.lookup(string(cl.Properties.Username), false)
	if ss == nil {
		return false
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.client != cl {
		return false
	}
	if p, ok := s.auth.(interface{ DeviceRevision(string) int64 }); ok && p.DeviceRevision(string(cl.Properties.Username)) != ss.revision {
		return false
	}
	return true
}
func (s *Server) sweep(now time.Time) {
	s.mu.Lock()
	snapshot := make(map[string]*session, len(s.sessions))
	for no, ss := range s.sessions {
		snapshot[no] = ss
	}
	s.mu.Unlock()
	for no, ss := range snapshot {
		if !ss.mu.TryLock() {
			continue
		}
		expired := ss.client == nil || !now.Before(ss.lastSeen.Add(ss.interval*time.Duration(s.cfg.OfflineGraceFactor)))
		if ss.checking || !ss.online || !expired {
			ss.mu.Unlock()
			continue
		}
		select {
		case s.slots <- struct{}{}:
		default:
			ss.mu.Unlock()
			continue
		}
		ss.checking = true
		ss.mu.Unlock()
		s.workers.Go(func() {
			defer func() { <-s.slots }()
			ss.mu.Lock()
			defer ss.mu.Unlock()
			defer func() { ss.checking = false }()
			if ss.client == nil || !now.Before(ss.lastSeen.Add(ss.interval*time.Duration(s.cfg.OfflineGraceFactor))) {
				if err := s.transition(no, ss, false); err != nil {
					s.log.Error("offline persistence", "device", no, "err", err)
				}
			}
		})
	}
}
func (s *Server) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.sweep(now)
		}
	}
}
