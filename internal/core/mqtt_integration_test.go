package core_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/access"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

func mqttString(s string) []byte {
	b := make([]byte, 2, len(s)+2)
	binary.BigEndian.PutUint16(b, uint16(len(s)))
	return append(b, s...)
}
func mqttPacket(kind byte, body []byte) []byte {
	out := []byte{kind}
	n := len(body)
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 128
		}
		out = append(out, b)
		if n == 0 {
			break
		}
	}
	return append(out, body...)
}
func readMQTT(c net.Conn) (byte, []byte, error) {
	one := make([]byte, 1)
	if _, err := io.ReadFull(c, one); err != nil {
		return 0, nil, err
	}
	header := one[0]
	n, mult := 0, 1
	for range 4 {
		if _, err := io.ReadFull(c, one); err != nil {
			return 0, nil, err
		}
		n += int(one[0]&127) * mult
		if one[0]&128 == 0 {
			body := make([]byte, n)
			_, err := io.ReadFull(c, body)
			return header, body, err
		}
		mult *= 128
	}
	return 0, nil, fmt.Errorf("bad packet")
}
func rawConnect(t *testing.T, addr string, version byte, will bool) (net.Conn, byte) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(3 * time.Second))
	flags := byte(0xc2)
	if will {
		flags |= 0x2c
	}
	body := append(mqttString("MQTT"), version, flags, 0, 30)
	if version == 5 {
		body = append(body, 0)
	}
	body = append(body, mqttString("one")...)
	if will {
		if version == 5 {
			body = append(body, 0)
		}
		body = append(body, mqttString("iolink/down/other/cmd")...)
		body = append(body, mqttString(`{"attack":true}`)...)
	}
	body = append(body, mqttString("one")...)
	body = append(body, mqttString("test-secret")...)
	if _, err = c.Write(mqttPacket(0x10, body)); err != nil {
		t.Fatal(err)
	}
	typ, response, err := readMQTT(c)
	if err != nil || typ != 0x20 || len(response) < 2 {
		t.Fatalf("CONNACK %x %v", typ, err)
	}
	return c, response[1]
}
func TestRealMQTTAuthenticationACLAndPersistence(t *testing.T) {
	svc, pool := setup(t)
	max := 26.0
	if _, err := svc.CreateRule(context.Background(), domain.AlarmRule{PondID: 1, Metric: "temperature", Max: &max, Level: domain.AlarmWarning}); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	broker := access.New(access.Config{MQTTAddr: addr, ReportInterval: time.Minute, OfflineGraceFactor: 3}, svc, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err = broker.Serve(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { broker.Close() })
	go broker.Run(t.Context())
	for _, version := range []byte{4, 5} {
		t.Run(fmt.Sprintf("will-v%d", version), func(t *testing.T) {
			c, code := rawConnect(t, addr, version, true)
			c.Close()
			if code == 0 {
				t.Fatal("cross-device retained will accepted")
			}
		})
	}
	if count(t, pool, "SELECT count(*) FROM devices WHERE status='online'") != 0 {
		t.Fatal("rejected will emitted online")
	}
	connect := func(id, secret string, want bool) mqtt.Client {
		opts := mqtt.NewClientOptions().AddBroker("tcp://" + addr).SetClientID(id).SetUsername("one").SetPassword(secret).SetAutoReconnect(false).SetConnectTimeout(2 * time.Second)
		c := mqtt.NewClient(opts)
		tok := c.Connect()
		if !tok.WaitTimeout(3 * time.Second) {
			t.Fatal("connect timeout")
		}
		if (tok.Error() == nil) != want {
			t.Fatalf("unexpected connection %q: %v", id, tok.Error())
		}
		if want {
			t.Cleanup(func() { c.Disconnect(100) })
		}
		return c
	}
	connect("mismatch", "test-secret", false)
	connect("one", "wrong", false)
	if count(t, pool, "SELECT count(*) FROM devices WHERE status='online'") != 0 {
		t.Fatal("failed auth emitted online")
	}
	c := connect("one", "test-secret", true)
	waitCount := func(q string, want int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if count(t, pool, q) == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("SQL expected count %d", want)
	}
	waitCount("SELECT count(*) FROM devices WHERE status='online'", 1)
	for _, topic := range []string{"iolink/down/one/cmd"} {
		tok := c.Subscribe(topic, 1, nil)
		if !tok.WaitTimeout(time.Second) || tok.Error() != nil {
			t.Fatal("own command subscription denied", tok.Error())
		}
	}
	for _, topic := range []string{"#", "iolink/down/+/cmd", "iolink/down/other/cmd"} {
		tok := c.Subscribe(topic, 1, nil)
		if !tok.WaitTimeout(time.Second) {
			t.Fatal("subscribe timeout")
		}
		if sub, ok := tok.(*mqtt.SubscribeToken); ok && sub.Result()[topic] != 0x80 {
			t.Fatalf("illegal subscription accepted %s", topic)
		}
	}
	publish := func(topic, payload string, valid bool) {
		t.Helper()
		tok := c.Publish(topic, 1, false, payload)
		done := tok.WaitTimeout(250 * time.Millisecond)
		if valid && (!done || tok.Error() != nil) {
			t.Fatal("valid publish failed", tok.Error())
		}
		if !valid {
			c.Disconnect(100)
			c = connect("one", "test-secret", true)
		}
	}
	publish("iolink/up/one/properties", `{"message_id":"mqtt-1","temperature":27,"TEMPERATURE":49,"signal":-65}`, true)
	waitCount("SELECT count(*) FROM sensor_data WHERE temperature=27 AND signal=-65", 1)
	waitCount("SELECT count(*) FROM alarms WHERE threshold=26 AND current_value=27 AND level='warning'", 1)
	publish("iolink/up/one/properties", `{"message_id":"mqtt-1","temperature":27}`, true)
	publish("iolink/up/other/properties", `{"temperature":28}`, false)
	publish("iolink/up/one/sub/forged/properties", `{"temperature":28}`, false)
	publish("iolink/up/one/properties", `{"Temperature":29}`, true)
	publish("iolink/up/one/properties", `{"temperature":"bad"}`, false)
	publish("iolink/up/one/properties", strings.Repeat(" ", 65537)+`{}`, false)
	publish("iolink/up/one/ack", `{"id":"test-command","ok":true}`, true)
	if count(t, pool, "SELECT count(*) FROM sensor_data") != 1 {
		t.Fatal("invalid packet wrote telemetry")
	}
	c2 := connect("one", "test-secret", true)
	waitCount("SELECT count(*) FROM devices WHERE status='online'", 1)
	c.Disconnect(0)
	tok := c2.Publish("iolink/up/one/properties", 1, false, `{"ph":7}`)
	if !tok.WaitTimeout(time.Second) || tok.Error() != nil {
		t.Fatal("replacement session failed")
	}
	waitCount("SELECT count(*) FROM sensor_data", 2)
	c2.Disconnect(100)
	waitCount("SELECT count(*) FROM devices WHERE status='online'", 0)
	// Actual MQTT 5 successful CONNECT and QoS1 publication as well as bad Will.
	c5, code := rawConnect(t, addr, 5, false)
	if code != 0 {
		t.Fatal("MQTT5 auth rejected")
	}
	body := append(mqttString("iolink/up/one/properties"), 0, 1, 0)
	body = append(body, []byte(`{"battery":90}`)...)
	c5.Write(mqttPacket(0x32, body))
	kind, ack, err := readMQTT(c5)
	if err != nil || kind != 0x40 || len(ack) < 2 {
		t.Fatal("MQTT5 publish acknowledgement", kind, err)
	}
	waitCount("SELECT count(*) FROM sensor_data", 3)
	c5.Write([]byte{0xe0, 0})
	c5.Close()
	waitCount("SELECT count(*) FROM devices WHERE status='online'", 0)
	execute(t, pool, "UPDATE devices SET disabled_at=now()")
	connect("one", "test-secret", false)
}
