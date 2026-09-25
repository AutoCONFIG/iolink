package access

import (
	"context"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"testing"
	"time"
)

func client(no string) *mqtt.Client {
	c := &mqtt.Client{ID: no}
	c.Properties.Username = []byte(no)
	return c
}
func TestAuthenticatedLifecycleWatchdogAndTakeover(t *testing.T) {
	for _, period := range []time.Duration{time.Minute, 5 * time.Minute} {
		t.Run(period.String(), func(t *testing.T) {
			f := &fakeHandler{}
			s := New(Config{ReportInterval: period, OfflineGraceFactor: 3}, f, fakeAuth{true}, testLogger())
			now := time.Now().UTC()
			s.now = func() time.Time { return now }
			a := client("one")
			h := newBrokerHook(s)
			bad := client("one")
			bad.ID = "different"
			if h.OnConnectAuthenticate(bad, packets.Packet{}) {
				t.Fatal("mismatch accepted")
			}
			if len(f.events) != 0 {
				t.Fatal("authentication emitted status")
			}
			h.OnSessionEstablished(a, packets.Packet{})
			now = now.Add(3 * period)
			s.sweep(now)
			s.workers.Wait()
			s.sweep(now)
			s.workers.Wait()
			if len(f.events) != 2 || f.events[0].Online != true || f.events[1].Online != false {
				t.Fatal(f.events)
			}
			if err := s.HandleReport("one", wire.Report{}); err != nil {
				t.Fatal(err)
			}
			if len(f.events) != 3 || !f.events[2].Online {
				t.Fatal("heartbeat did not restore online")
			}
			b := client("one")
			h.OnSessionEstablished(b, packets.Packet{})
			before := len(f.events)
			h.OnDisconnect(a, nil, false)
			if len(f.events) != before {
				t.Fatal("old client disconnected new session")
			}
			h.OnDisconnect(b, nil, false)
			h.OnDisconnect(b, nil, false)
			s.sweep(now.Add(20 * period))
			s.workers.Wait()
			if len(f.events) != before+1 || f.events[len(f.events)-1].Online {
				t.Fatal("repeated disconnect")
			}
		})
	}
}
func TestPayloadValidationAndACL(t *testing.T) {
	f := &fakeHandler{}
	s := New(Config{}, f, fakeAuth{true}, testLogger())
	h := newBrokerHook(s)
	cl := client("one")
	h.OnSessionEstablished(cl, packets.Packet{})
	for _, topic := range []string{"iolink/down/one/#", "iolink/down/+/cmd", "iolink/down/two/cmd", "iolink/up/one/properties"} {
		if h.OnACLCheck(cl, topic, false) {
			t.Fatal(topic)
		}
	}
	cases := []struct {
		payload string
		valid   bool
		count   int
	}{
		{`{"temperature":26,"signal":-120,"message_id":"a"}`, true, 2},
		{`{"temperature":50,"battery":100,"signal":0}`, true, 3},
		{`{"temperature":51,"ph":7,"unknown":3}`, true, 1},
		{`{"temperature":null}`, true, 0},
		{`{"Temperature":27}`, true, 0}, {`{"temperature":27,"TEMPERATURE":49}`, true, 1}, {`{"unknown":3}`, true, 0}, {`{}`, true, 0},
		{`null`, false, 0}, {`[]`, false, 0}, {`{`, false, 0}, {`{"temperature":"26"}`, false, 0}, {`{"signal":-65.5}`, false, 0}, {`{"message_id":""}`, false, 0},
	}
	for _, tt := range cases {
		f.events = nil
		_, err := h.OnPublish(cl, packets.Packet{TopicName: "iolink/up/one/properties", Payload: []byte(tt.payload), FixedHeader: packets.FixedHeader{Qos: 1}})
		if (err == nil) != tt.valid {
			t.Fatalf("%s: %v", tt.payload, err)
		}
		count := 0
		for _, e := range f.events {
			if e.Kind == event.KindProperties {
				count = len(e.Properties)
			}
		}
		if count != tt.count {
			t.Fatalf("%s count=%d", tt.payload, count)
		}
	}
	for _, pk := range []packets.Packet{
		{TopicName: "iolink/up/one/properties", Payload: []byte("{\"message_id\":\"\xff\",\"temperature\":26}"), FixedHeader: packets.FixedHeader{Qos: 1}},
		{TopicName: "iolink/up/one/properties", Payload: make([]byte, 65537), FixedHeader: packets.FixedHeader{Qos: 1}},
		{TopicName: "iolink/up/two/properties", Payload: []byte(`{}`), FixedHeader: packets.FixedHeader{Qos: 1}},
		{TopicName: "iolink/up/one/sub/x/properties", Payload: []byte(`{}`), FixedHeader: packets.FixedHeader{Qos: 1}},
		{TopicName: "iolink/up/one/properties", Payload: []byte(`{}`), FixedHeader: packets.FixedHeader{Qos: 1, Retain: true}},
	} {
		if _, err := h.OnPublish(cl, pk); err == nil {
			t.Fatal("invalid packet accepted")
		}
	}
	if h.Provides(mqtt.OnConnect) {
		t.Fatal("pre-auth connect hook enabled")
	}
}

type blockedHandler struct {
	entered chan struct{}
	release chan struct{}
}

func (h *blockedHandler) HandleEvent(e event.Event) error {
	if e.DeviceNo == "slow" {
		select {
		case h.entered <- struct{}{}:
		default:
		}
		<-h.release
	}
	return nil
}
func TestSlowDeviceDoesNotBlockOthersOrCancellation(t *testing.T) {
	h := &blockedHandler{make(chan struct{}, 1), make(chan struct{})}
	s := New(Config{}, h, fakeAuth{true}, testLogger())
	a, b := client("slow"), client("healthy")
	done := make(chan struct{})
	go func() { s.connected(a); close(done) }()
	<-h.entered
	healthy := make(chan struct{})
	go func() {
		s.connected(b)
		if !s.isCurrent(b) {
			t.Error("healthy client missing")
		}
		s.disconnected(b)
		close(healthy)
	}()
	select {
	case <-healthy:
	case <-time.After(time.Second):
		close(h.release)
		t.Fatal("one slow device blocked all clients")
	}
	ctx, cancel := context.WithCancel(context.Background())
	exited := make(chan struct{})
	go func() { s.Run(ctx); close(exited) }()
	cancel()
	select {
	case <-exited:
	case <-time.After(time.Second):
		close(h.release)
		t.Fatal("Run cancellation blocked")
	}
	close(h.release)
	<-done
}

func TestAllDocumentedMetricBoundaries(t *testing.T) {
	f := &fakeHandler{}
	s := New(Config{}, f, fakeAuth{true}, testLogger())
	h := newBrokerHook(s)
	cl := client("one")
	h.OnSessionEstablished(cl, packets.Packet{})
	cases := []struct {
		key       string
		low, high float64
	}{{"temperature", 0, 50}, {"dissolved_oxygen", 0, 20}, {"ph", 0, 14}, {"turbidity", 0, 1000}, {"salinity", 0, 50}, {"battery", 0, 100}, {"signal", -120, 0}}
	for _, c := range cases {
		for _, value := range []float64{c.low - 1, c.low, c.high, c.high + 1} {
			f.events = nil
			payload := fmt.Sprintf(`{"%s":%g}`, c.key, value)
			if _, err := h.OnPublish(cl, packets.Packet{TopicName: "iolink/up/one/properties", Payload: []byte(payload), FixedHeader: packets.FixedHeader{Qos: 1}}); err != nil {
				t.Fatal(err)
			}
			var values map[string]float64
			for _, e := range f.events {
				if e.Kind == event.KindProperties {
					values = e.Properties
				}
			}
			_, present := values[c.key]
			if present != (value >= c.low && value <= c.high) {
				t.Fatal("boundary", payload, values)
			}
		}
	}
}
