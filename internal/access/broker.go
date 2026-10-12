package access

import (
	"encoding/json"
	"git.hyhy.fun/rsplab/iolink/internal/ingestion"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"strings"
	"unicode/utf8"
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
func (h *brokerHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	if cl == nil || pk.Connect.WillFlag {
		return false
	}
	// Identity comes solely from the username pair; the client ID is an
	// arbitrary transport session key (docs/mqtt-spec.md).
	no := string(cl.Properties.Username)
	if no == "" || strings.ContainsAny(no, "/+#") || !ValidClientID(cl.ID) {
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
	if !h.srv.auth.Authenticate(no, string(pk.Connect.Password)) {
		return false
	}
	if p, ok := h.srv.auth.(interface{ DeviceRevision(string) int64 }); ok && p.DeviceRevision(no) < 0 {
		return false
	}
	return true
}
func (h *brokerHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return cl != nil && h.srv.isCurrent(cl) && CheckACL(string(cl.Properties.Username), topic, write)
}
func (h *brokerHook) OnSessionEstablished(cl *mqtt.Client, _ packets.Packet) { h.srv.connected(cl) }
func (h *brokerHook) OnDisconnect(cl *mqtt.Client, _ error, _ bool)          { h.srv.disconnected(cl) }
func (h *brokerHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	reject := func(reason string) (packets.Packet, error) {
		MetricRejected.WithLabelValues(reason).Inc()
		return pk, packets.ErrRejectPacket
	}
	if cl == nil || !h.srv.isCurrent(cl) {
		return reject("session")
	}
	no, kind, err := ParseUplinkTopic(pk.TopicName)
	if err != nil || no != string(cl.Properties.Username) {
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
			return reject("dispatch")
		}
		if kind == "generic" {
			if err := h.srv.HandleGenericReport(no, raw); err != nil {
				return reject("dispatch")
			}
			return pk, nil
		}
		for key := range raw {
			if _, _, ok := ingestion.MetricRange(key); !ok && key != "message_id" {
				MetricRejected.WithLabelValues("unknown_field").Inc()
				delete(raw, key)
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
		if err := h.srv.HandleReport(no, r); err != nil {
			h.srv.log.Error("properties rejected", "device", no, "err", err)
			return reject("dispatch")
		}
	} else {
		var ack struct {
			ID    string `json:"id"`
			OK    *bool  `json:"ok"`
			Error string `json:"error"`
		}
		if json.Unmarshal(pk.Payload, &ack) != nil || ack.ID == "" || len(ack.ID) > 128 || ack.OK == nil || (!*ack.OK && ack.Error == "") {
			return reject("ack")
		}
		h.srv.log.Info("device ack accepted", "device", no) // never log arbitrary payload/credentials
	}
	return pk, nil
}
