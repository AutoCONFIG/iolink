package access

import (
	"encoding/json"
	"strings"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	"git.hyhy.fun/rsplab/iolink/internal/wire"
)

// brokerHook wires authentication, ACL and payload dispatch into the
// embedded mochi-mqtt broker. It is the ONLY place that knows both the
// MQTT layer and the internal event path.
type brokerHook struct {
	mqtt.HookBase
	srv *Server
}

func newBrokerHook(s *Server) *brokerHook { return &brokerHook{srv: s} }

func (h *brokerHook) ID() string { return "iolink-access-hook" }

func (h *brokerHook) Provides(b byte) bool {
	switch b {
	case mqtt.OnConnectAuthenticate, mqtt.OnACLCheck, mqtt.OnPublish, mqtt.OnConnect, mqtt.OnDisconnect:
		return true
	}
	return false
}

// OnConnectAuthenticate checks the device triple: username = device_no.
func (h *brokerHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	user := ""
	if cl != nil {
		user = string(cl.Properties.Username)
	}
	ok := h.srv.auth.Authenticate(user, string(pk.Connect.Password))
	if !ok {
		h.srv.log.Warn("device auth rejected", "device", user)
	}
	return ok
}

// OnACLCheck keeps every client inside its own topic sandbox.
func (h *brokerHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	ok := CheckACL(string(cl.Properties.Username), topic, write)
	if !ok {
		h.srv.log.Warn("acl denied", "device", string(cl.Properties.Username), "topic", topic, "write", write)
	}
	return ok
}

// OnConnect enqueues an online event (fan-out happens in Server.Run).
func (h *brokerHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	if len(cl.Properties.Username) > 0 {
		select {
		case h.srv.connectCh <- string(cl.Properties.Username):
		default:
		}
	}
	return nil
}

func (h *brokerHook) OnDisconnect(cl *mqtt.Client, err error, _ bool) {
	if len(cl.Properties.Username) > 0 {
		select {
		case h.srv.disconnectCh <- string(cl.Properties.Username):
		default:
		}
	}
}

// OnPublish validates and dispatches an uplink payload to core.
func (h *brokerHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	deviceNo, kind, err := ParseUplinkTopic(pk.TopicName)
	if err != nil {
		h.srv.log.Warn("bad topic publish", "topic", pk.TopicName, "client", cl.ID)
		return pk, nil // drop; ACL already blocks foreign topics
	}
	if deviceNo != string(cl.Properties.Username) {
		h.srv.log.Warn("device impersonation attempt", "topic", pk.TopicName, "client", cl.ID)
		return pk, nil
	}
	switch kind {
	case "properties":
		var r wire.Report
		if err := json.Unmarshal(pk.Payload, &r); err != nil {
			h.srv.log.Warn("bad properties payload", "device", deviceNo, "err", err)
			return pk, nil
		}
		if err := h.srv.HandleReport(deviceNo, r); err != nil {
			h.srv.log.Error("dispatch properties", "device", deviceNo, "err", err)
		}
	case "ack":
		h.srv.log.Info("device ack", "device", deviceNo, "payload", strings.TrimSpace(string(pk.Payload)))
	}
	return pk, nil
}
