package access

import (
	"encoding/json"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/ingestion"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

type brokerHook struct {
	mqtt.HookBase
	srv *Server
}

func newBrokerHook(s *Server) *brokerHook { return &brokerHook{srv: s} }
func (h *brokerHook) ID() string          { return "iolink-access-hook" }
func (h *brokerHook) Provides(b byte) bool {
	switch b {
	case mqtt.OnConnectAuthenticate, mqtt.OnACLCheck, mqtt.OnPublish, mqtt.OnSessionEstablished, mqtt.OnDisconnect:
		return true
	}
	return false
}

func (h *brokerHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) (accepted bool) {
	defer func() {
		outcome := "rejected"
		if accepted {
			outcome = "accepted"
		}
		h.srv.log.Debug("mqtt authentication completed", "ingest_id", observability.NewIngestID(), "stage", "mqtt_authentication", "outcome", outcome)
	}()
	if cl == nil || pk.Connect.WillFlag {
		return false
	}
	// ThingsPanel-style issued credentials: the username is a platform token
	// resolved to the owning device_no; the client ID is an arbitrary
	// transport session key (docs/mqtt-spec.md).
	username := string(cl.Properties.Username)
	if username == "" || !ValidClientID(cl.ID) {
		return false
	}
	// Clean sessions only: a persistent session keyed by an arbitrary client
	// ID would let one device resume another device's subscriptions and
	// queued downlink messages. This covers v3 CleanSession and v5 both
	// Clean Start and Session Expiry Interval (inheritClientSession hands
	// over subscriptions and inflight QoS1 messages on CleanStart=0).
	if !pk.Connect.Clean {
		return false
	}
	if pk.ProtocolVersion >= 5 && pk.Properties.SessionExpiryIntervalFlag && pk.Properties.SessionExpiryInterval > 0 {
		return false
	}
	deviceNo, ok := h.srv.auth.Resolve(username, string(pk.Connect.Password))
	if !ok {
		return false
	}
	if p, ok := h.srv.auth.(interface{ DeviceRevision(string) int64 }); ok && p.DeviceRevision(deviceNo) < 0 {
		return false
	}
	return true
}

func (h *brokerHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	if cl == nil || !h.srv.isCurrent(cl) {
		return false
	}
	no, ok := h.srv.deviceFor(cl)
	return ok && CheckACL(no, topic, write)
}

func (h *brokerHook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	h.srv.connected(cl, string(pk.Connect.Password))
}
func (h *brokerHook) OnDisconnect(cl *mqtt.Client, _ error, _ bool) { h.srv.disconnected(cl) }
func (h *brokerHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	ingestID := observability.NewIngestID()
	h.srv.log.Debug("mqtt message received", "ingest_id", ingestID, "stage", "mqtt_ingress", "outcome", "started", "protocol", "mqtt", "qos", pk.FixedHeader.Qos, "retain", pk.FixedHeader.Retain)
	reject := func(reason string, cause ...error) (packets.Packet, error) {
		MetricRejected.WithLabelValues(reason).Inc()
		stage := "mqtt_validation"
		if reason == "dispatch" {
			stage = "mqtt_dispatch"
		}
		attrs := []any{"ingest_id", ingestID, "stage", stage, "outcome", "rejected", "error_code", reason}
		if len(cause) > 0 {
			attrs = append(attrs, "err", cause[0])
		}
		h.srv.log.Warn("mqtt message rejected", attrs...)
		return pk, packets.ErrRejectPacket
	}
	if cl == nil || !h.srv.isCurrent(cl) {
		return reject("session")
	}
	no, ok := h.srv.deviceFor(cl)
	if !ok {
		return reject("session")
	}
	noInTopic, kind, err := ParseUplinkTopic(pk.TopicName)
	if err != nil || noInTopic != no {
		return reject("topic")
	}
	if len(pk.Payload) > 65536 || pk.FixedHeader.Retain || pk.FixedHeader.Qos != 1 {
		return reject("envelope")
	}
	if !utf8.Valid(pk.Payload) {
		return reject("json")
	}
	if kind == "properties" {
		var raw map[string]json.RawMessage
		if json.Unmarshal(pk.Payload, &raw) != nil || raw == nil {
			return reject("json")
		}
		kind, kindErr := h.srv.productKind(no)
		if kindErr != nil {
			return reject("dispatch", kindErr)
		}
		if kind == "generic" {
			h.srv.log.Debug("mqtt dispatch started", "ingest_id", ingestID, "stage", "mqtt_dispatch", "outcome", "started", "kind", "properties")
			if err := h.srv.handleGenericReport(no, raw, ingestID); err != nil {
				return reject("dispatch", err)
			}
			h.srv.log.Debug("mqtt message dispatched", "ingest_id", ingestID, "stage", "mqtt_dispatch", "outcome", "accepted", "kind", "properties", "field_count", len(raw))
			return pk, nil
		}
		dropped := 0
		for key := range raw {
			if _, _, ok := ingestion.MetricRange(key); !ok && key != "message_id" {
				MetricRejected.WithLabelValues("unknown_field").Inc()
				delete(raw, key)
				dropped++
			}
		}
		var r wire.Report
		clean, err := json.Marshal(raw)
		if err != nil {
			return reject("json")
		}
		if json.Unmarshal(clean, &r) != nil {
			return reject("type")
		}
		h.srv.log.Debug("mqtt dispatch started", "ingest_id", ingestID, "stage", "mqtt_dispatch", "outcome", "started", "rejected_count", dropped)
		if err := h.srv.handleReport(no, r, ingestID); err != nil {
			return reject("dispatch", err)
		}
		h.srv.log.Debug("mqtt message dispatched", "ingest_id", ingestID, "stage", "mqtt_dispatch", "outcome", "accepted", "kind", "properties")
	} else {
		var ack struct {
			ID    string `json:"id"`
			OK    *bool  `json:"ok"`
			Error string `json:"error"`
		}
		if json.Unmarshal(pk.Payload, &ack) != nil || ack.ID == "" || len(ack.ID) > 128 || ack.OK == nil || (!*ack.OK && ack.Error == "") {
			return reject("ack")
		}
		h.srv.log.Debug("device ack accepted", "ingest_id", ingestID, "stage", "mqtt_dispatch", "outcome", "accepted", "kind", "ack") // never log arbitrary payload/credentials
	}
	return pk, nil
}
