package access

import (
	"errors"
	"fmt"
	"strings"
)

// Authenticator verifies the device triple on MQTT connect.
// core provides the production impl (devices.secret_hash); tests use fakes.
type Authenticator interface {
	Authenticate(deviceNo, secret string) bool
}

// NoopAuth accepts everything — DEV ONLY, logs loudly via Server logger.
type NoopAuth struct{}

func (NoopAuth) Authenticate(string, string) bool { return true }

// Topic helpers: the single place that knows the topic grammar
// (docs/mqtt-spec.md). Pure functions → unit-testable without a broker.

var ErrBadTopic = errors.New("topic outside iolink grammar")

// ParseUplinkTopic validates "iolink/up/{device_no}/{kind}" where kind is
// "properties" or "ack", and returns the device number. write=true for
// downlink topics ("iolink/down/{device_no}/cmd").
func ParseUplinkTopic(topic string) (deviceNo, kind string, err error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "iolink" || parts[1] != "up" {
		return "", "", fmt.Errorf("%w: %q", ErrBadTopic, topic)
	}
	if parts[2] == "" || strings.ContainsAny(parts[2], "+#") {
		return "", "", fmt.Errorf("%w: empty device_no in %q", ErrBadTopic, topic)
	}
	switch parts[3] {
	case "properties", "ack":
		return parts[2], parts[3], nil
	default:
		return "", "", fmt.Errorf("%w: unknown kind %q", ErrBadTopic, parts[3])
	}
}

// DownlinkTopic builds the command topic for a device (platform -> device).
func DownlinkTopic(deviceNo string) string { return "iolink/down/" + deviceNo + "/cmd" }

// CheckACL enforces the per-device topic sandbox:
// publish allowed only on iolink/up/{own}/#, subscribe only iolink/down/{own}/#.
func CheckACL(deviceNo, topic string, write bool) bool {
	if deviceNo == "" || strings.ContainsAny(deviceNo, "/+#") {
		return false
	}
	if write {
		d, _, err := ParseUplinkTopic(topic)
		return err == nil && d == deviceNo
	}
	return topic == DownlinkTopic(deviceNo)
}
