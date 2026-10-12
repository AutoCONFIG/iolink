package access

import (
	"context"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"strings"
	"testing"
	"time"
)

func client(no string) *mqtt.Client {
	c := &mqtt.Client{ID: no}
	c.Properties.Username = []byte(no)
	return c
}

func TestConnectAuthGenericClientID(t *testing.T) {
	f := &fakeHandler{}
	s := New(Config{}, f, fakeAuth{true}, testLogger())
	h := newBrokerHook(s)
	good := packets.Packet{ProtocolVersion: 4, Connect: packets.ConnectParams{Clean: true, Password: []byte("s")}}

	// Any client ID is accepted: identity is the username pair only.
	for _, id := range []string{"one", "mqtt_20842687-727", "设备-1", "client.example-01"} {
		cl := client("one")
		cl.ID = id
		if !h.OnConnectAuthenticate(cl, good) {
			t.Fatalf("client id %q rejected", id)
		}
	}
	// Usernames are opaque issued tokens, free of topic grammar.
	for _, name := range []string{"iolink-one-seed01", "de/v1", "a+user"} {
		cl := client(name)
		cl.ID = "fixed-client"
		if !h.OnConnectAuthenticate(cl, good) {
			t.Fatalf("username %q rejected", name)
		}
	}

	reject := func(name string, cl *mqtt.Client, pk packets.Packet) {
		t.Helper()
		if h.OnConnectAuthenticate(cl, pk) {
			t.Fatalf("%s accepted", name)
		}
	}
	reject("empty username", client(""), good)
	wildcardID := client("one")
	wildcardID.ID = "a+"
	reject("wildcard client id", wildcardID, good)
	longID := client("one")
	longID.ID = strings.Repeat("a", 129)
	reject("oversized client id", longID, good)
	badID := client("one")
	badID.ID = "\xff"
	reject("non-utf8 client id", badID, good)
	emptyID := client("one")
	emptyID.ID = ""
	reject("empty client id", emptyID, good)

	persistent := good
	persistent.Connect.Clean = false
	reject("v3 persistent session", client("one"), persistent)

	v5expiry := good
	v5expiry.ProtocolVersion = 5
	v5expiry.Properties = packets.Properties{SessionExpiryIntervalFlag: true, SessionExpiryInterval: 30}
	reject("v5 persistent session", client("one"), v5expiry)

	v5resumed := good
	v5resumed.ProtocolVersion = 5
	v5resumed.Connect.Clean = false
	reject("v5 clean start 0 without expiry property", client("one"), v5resumed)

	v5resumedZero := v5resumed
	v5resumedZero.Properties = packets.Properties{SessionExpiryIntervalFlag: true}
	reject("v5 clean start 0 with zero expiry", client("one"), v5resumedZero)

	// v5 clean start with explicit zero expiry is a clean session.
	v5zero := good
	v5zero.ProtocolVersion = 5
	v5zero.Properties = packets.Properties{SessionExpiryIntervalFlag: true}
	if !h.OnConnectAuthenticate(client("one"), v5zero) {
		t.Fatal("v5 clean start with zero expiry rejected")
	}

	will := good
	will.Connect.WillFlag = true
	reject("will flag", client("one"), will)

	denied := New(Config{}, f, fakeAuth{false}, testLogger())
	hDenied := newBrokerHook(denied)
	if hDenied.OnConnectAuthenticate(client("one"), good) {
		t.Fatal("wrong secret accepted")
	}
	unavailable := New(Config{}, f, unavailableRevisionAuth{}, testLogger())
	hUnavailable := newBrokerHook(unavailable)
	if hUnavailable.OnConnectAuthenticate(client("one"), good) {
		t.Fatal("unavailable revision accepted")
	}
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
			good := packets.Packet{ProtocolVersion: 4, Connect: packets.ConnectParams{Clean: true, Password: []byte("s")}}
			generic := client("one")
			generic.ID = "mqtt_20842687-727"
			if !h.OnConnectAuthenticate(generic, good) {
				t.Fatal("arbitrary client id with valid pair rejected")
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

type unavailableRevisionAuth struct{}

func (unavailableRevisionAuth) Resolve(username, _ string) (string, bool) { return username, true }
func (unavailableRevisionAuth) DeviceRevision(string) int64               { return -1 }

func TestResolveFailureAtEstablishmentFailsClosed(t *testing.T) {
	f := &fakeHandler{}
	s := New(Config{}, f, fakeAuth{false}, testLogger())
	cl := client("one")
	h := newBrokerHook(s)
	h.OnSessionEstablished(cl, packets.Packet{})
	if s.isCurrent(cl) {
		t.Fatal("unresolved session must not pass ACL")
	}
	s.mu.Lock()
	_, mapped := s.deviceOf[cl]
	s.mu.Unlock()
	if mapped {
		t.Fatal("unresolved client must not be mapped")
	}
	if len(f.events) != 0 {
		t.Fatal("unresolved session emitted status")
	}
}

func TestRevisionLookupFailureFailsClosed(t *testing.T) {
	f := &fakeHandler{}
	s := New(Config{}, f, unavailableRevisionAuth{}, testLogger())
	cl := client("one")
	h := newBrokerHook(s)
	if h.OnConnectAuthenticate(cl, packets.Packet{Connect: packets.ConnectParams{Password: []byte("secret")}}) {
		t.Fatal("unavailable revision must reject CONNECT")
	}
	h.OnSessionEstablished(cl, packets.Packet{})
	if s.isCurrent(cl) {
		t.Fatal("session with unavailable revision must not pass ACL")
	}
	if len(f.events) != 0 {
		t.Fatal("unavailable revision must not emit online")
	}
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
	go func() { s.connected(a, ""); close(done) }()
	<-h.entered
	healthy := make(chan struct{})
	go func() {
		s.connected(b, "")
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
