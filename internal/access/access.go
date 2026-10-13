// Package access authenticates MQTT devices and normalizes incoming events.
package access

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/ingestion"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

func (s *Server) HandleGenericReport(no string, raw map[string]json.RawMessage) error {
	return s.handleGenericReport(no, raw, observability.NewIngestID())
}

func (s *Server) handleGenericReport(no string, raw map[string]json.RawMessage, ingestID string) error {
	var messageID string
	if value, ok := raw["message_id"]; ok {
		if err := json.Unmarshal(value, &messageID); err != nil {
			return errors.New("invalid message_id")
		}
		delete(raw, "message_id")
	}
	e := event.Event{IngestID: ingestID, Kind: event.KindProperties, DeviceNo: no, MessageID: messageID, Ts: s.now().UTC(), GenericProperties: raw}
	if ss := s.lookup(no, false); ss != nil {
		ss.mu.Lock()
		if ss.client != nil {
			ss.lastSeen = e.Ts
			if err := s.transitionWithIngestID(no, ss, true, ingestID); err != nil {
				ss.mu.Unlock()
				return err
			}
		}
		ss.mu.Unlock()
	}
	return s.handler.HandleEvent(e)
}

func (s *Server) genericProduct(no string) bool {
	provider, ok := s.auth.(interface{ GenericProduct(string) bool })
	return ok && provider.GenericProduct(no)
}

func (s *Server) productKind(no string) (string, error) {
	if provider, ok := s.auth.(interface{ ProductKind(string) (string, error) }); ok {
		return provider.ProductKind(no)
	}
	if s.genericProduct(no) {
		return "generic", nil
	}
	return "water", nil
}

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
	deviceOf map[*mqtt.Client]string // resolved device_no per live MQTT client
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
	s := &Server{cfg: cfg, handler: handler, auth: auth, log: log, sessions: map[string]*session{}, deviceOf: map[*mqtt.Client]string{}, now: time.Now, slots: make(chan struct{}, 16)}
	caps := mqtt.NewDefaultServerCapabilities()
	// Bound allocations before decoding, allowing MQTT headers beyond 64KiB payload.
	caps.MaximumPacketSize = 65536 + 1024
	s.broker = mqtt.New(&mqtt.Options{InlineClient: true, Capabilities: caps, Logger: log})
	return s
}

type denyAuth struct{}

func (denyAuth) Resolve(string, string) (string, bool) { return "", false }

// deviceFor returns the resolved device_no of a live client session.
func (s *Server) deviceFor(cl *mqtt.Client) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	no, ok := s.deviceOf[cl]
	return no, ok
}

func (s *Server) setDeviceFor(cl *mqtt.Client, no string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceOf[cl] = no
}

func (s *Server) Serve() error {
	if err := s.broker.AddHook(newBrokerHook(s), nil); err != nil {
		return err
	}
	if err := s.broker.AddListener(listeners.NewTCP(listeners.Config{ID: "iolink-mqtt", Address: s.cfg.MQTTAddr})); err != nil {
		return err
	}
	return s.broker.Serve()
}
func (s *Server) Close() error { return s.CloseContext(context.Background()) }

func (s *Server) CloseContext(ctx context.Context) error {
	err := s.broker.Close()
	done := make(chan struct{})
	go func() {
		s.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		if err == nil {
			err = ctx.Err()
		}
	}
	return err
}

func (s *Server) HandleReport(no string, r wire.Report) error {
	return s.handleReport(no, r, observability.NewIngestID())
}

func (s *Server) handleReport(no string, r wire.Report, ingestID string) error {
	e := event.Event{IngestID: ingestID, Kind: event.KindProperties, DeviceNo: no, Ts: s.now().UTC(), Properties: map[string]float64{}}
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
		low, high, ok := ingestion.MetricRange(k)
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < low || v > high {
			delete(e.Properties, k)
			MetricRejected.WithLabelValues("range").Inc()
			s.log.Warn("field out of range", "device", no, "field", k, "ingest_id", ingestID, "stage", "mqtt_validation")
		}
	}
	// Valid authenticated packets are heartbeats even when they contain no samples.
	if ss := s.lookup(no, false); ss != nil {
		ss.mu.Lock()
		if ss.client != nil {
			ss.lastSeen = e.Ts
			if err := s.transitionWithIngestID(no, ss, true, ingestID); err != nil {
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
	return s.transitionWithIngestID(no, ss, online, observability.NewIngestID())
}

func (s *Server) transitionWithIngestID(no string, ss *session, online bool, ingestID string) error {
	if ss.online == online && !online {
		return nil
	}
	if err := s.handler.HandleEvent(event.Event{IngestID: ingestID, Kind: event.KindStatusChange, DeviceNo: no, Ts: s.now().UTC(), Online: online}); err != nil {
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

func (s *Server) connected(cl *mqtt.Client, password string) {
	// Re-resolve at session establishment: the deviceOf mapping below drives
	// every ACL decision for this connection, so it must not trust
	// CONNACK-time state alone (the device may have been disabled in between).
	no, ok := s.auth.Resolve(string(cl.Properties.Username), password)
	if !ok {
		// The broker already CONNACKed this client; without a mapping every
		// ACL check would fail forever, so retire the zombie now.
		go s.retireClient(cl)
		return
	}
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
	if p, ok := s.auth.(interface{ DeviceRevision(string) int64 }); ok {
		revision := p.DeviceRevision(no)
		if revision < 0 {
			return
		}
		ss.revision = revision
	}
	old := ss.client
	s.setDeviceFor(cl, no)
	ss.client = cl
	ss.interval = interval
	ss.lastSeen = s.now()
	if err := s.transition(no, ss, true); err != nil {
		s.log.Error("online persistence", "device", no, "err", err)
	}
	// Client IDs are arbitrary, so a second live connection for the same
	// device_no is not a broker-level takeover; retire the previous one so a
	// device truly has a single active session.
	if old != nil && old != cl && !old.IsTakenOver() && !old.Closed() {
		go s.retireClient(old)
	}
}

// retireClient kicks a previous connection of the same device with a graceful
// Session Taken Over DISCONNECT. The write can block indefinitely on a
// stalled peer with keepalive=0, so the TCP connection is force-closed when
// it does not finish promptly.
func (s *Server) retireClient(old *mqtt.Client) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.broker.DisconnectClient(old, packets.ErrSessionTakenOver)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		old.Stop(packets.ErrSessionTakenOver)
	}
}

func (s *Server) disconnected(cl *mqtt.Client) {
	no, ok := s.deviceFor(cl)
	s.mu.Lock()
	delete(s.deviceOf, cl)
	s.mu.Unlock()
	if !ok {
		return
	}
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
	no, ok := s.deviceFor(cl)
	if !ok {
		return false
	}
	ss := s.lookup(no, false)
	if ss == nil {
		return false
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.client != cl {
		return false
	}
	if p, ok := s.auth.(interface{ DeviceRevision(string) int64 }); ok {
		revision := p.DeviceRevision(no)
		if revision < 0 || revision != ss.revision {
			return false
		}
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
