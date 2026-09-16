package access

import (
	"testing"
	"time"

	packets "github.com/mochi-mqtt/server/v2/packets"

	iolinkcontracts "git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/wire"
)

type fakeHandler struct{ events []iolinkcontracts.Event }

func (f *fakeHandler) HandleEvent(e iolinkcontracts.Event) error {
	f.events = append(f.events, e)
	return nil
}

type fakeAuth struct{ ok bool }

func (f fakeAuth) Authenticate(string, string) bool { return f.ok }

func f64(v float64) *float64 { return &v }

func TestHandleReportNormalizes(t *testing.T) {
	fh := &fakeHandler{}
	s := New(Config{ReportInterval: 60}, fh, nil, testLogger())
	err := s.HandleReport("dev-001", wire.Report{
		Temperature: f64(27.5), DO: f64(6.8), PH: f64(7.9),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fh.events) != 1 {
		t.Fatalf("want 1 event, got %d", len(fh.events))
	}
	e := fh.events[0]
	if e.Kind != iolinkcontracts.KindProperties || e.DeviceNo != "dev-001" {
		t.Fatalf("bad envelope: %+v", e)
	}
	if e.Properties["temperature"] != 27.5 || e.Properties["dissolved_oxygen"] != 6.8 {
		t.Fatalf("bad properties: %+v", e.Properties)
	}
	if _, ok := e.Properties["turbidity"]; ok {
		t.Fatal("absent field must stay absent")
	}
}

func TestStatusChange(t *testing.T) {
	fh := &fakeHandler{}
	s := New(Config{ReportInterval: 60}, fh, nil, testLogger())
	s.connectCh <- "dev-001"
	go s.Run(t.Context())
	time.Sleep(50 * time.Millisecond)
	s.disconnectCh <- "dev-001"
	time.Sleep(50 * time.Millisecond)
	if len(fh.events) < 2 ||
		fh.events[0].Kind != iolinkcontracts.KindStatusChange ||
		fh.events[0].Online != true ||
		fh.events[1].Online != false {
		t.Fatalf("bad status events: %+v", fh.events)
	}
}

func TestParseUplinkTopic(t *testing.T) {
	cases := []struct {
		topic string
		no    string
		kind  string
		ok    bool
	}{
		{"iolink/up/dev-001/properties", "dev-001", "properties", true},
		{"iolink/up/dev-002/ack", "dev-002", "ack", true},
		{"iolink/up/properties", "", "", false},
		{"iolink/down/dev-001/cmd", "", "", false},
		{"other/up/dev-001/properties", "", "", false},
		{"iolink/up/dev-001/bogus", "", "", false},
	}
	for _, c := range cases {
		no, kind, err := ParseUplinkTopic(c.topic)
		if c.ok && (err != nil || no != c.no || kind != c.kind) {
			t.Fatalf("topic %q: got (%q,%q,%v)", c.topic, no, kind, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("topic %q should fail", c.topic)
		}
	}
}

func TestCheckACL(t *testing.T) {
	if !CheckACL("dev-001", "iolink/up/dev-001/properties", true) {
		t.Fatal("own properties publish must be allowed")
	}
	if !CheckACL("dev-001", "iolink/up/dev-001/ack", true) {
		t.Fatal("own ack publish must be allowed")
	}
	if CheckACL("dev-001", "iolink/up/dev-002/properties", true) {
		t.Fatal("foreign topic publish must be denied")
	}
	if !CheckACL("dev-001", DownlinkTopic("dev-001"), false) {
		t.Fatal("own downlink subscribe must be allowed")
	}
	if CheckACL("dev-001", DownlinkTopic("dev-002"), false) {
		t.Fatal("foreign downlink subscribe must be denied")
	}
}

func TestAuthHook(t *testing.T) {
	fh := &fakeHandler{}
	s := New(Config{ReportInterval: 60}, fh, fakeAuth{ok: false}, testLogger())
	h := newBrokerHook(s)
	if h.OnConnectAuthenticate(nil, packets.Packet{Connect: packets.ConnectParams{Password: []byte("x")}}) {
		t.Fatal("rejected device must not authenticate")
	}
}

func TestRangeValidation(t *testing.T) {
	fh := &fakeHandler{}
	s := New(Config{ReportInterval: 60}, fh, nil, testLogger())
	_ = s.HandleReport("dev-001", wire.Report{
		Temperature: f64(99),  // 超范围 → 丢弃
		DO:          f64(6.8), // 合法 → 保留
		PH:          f64(-1),  // 超范围 → 丢弃
		Battery:     f64(95),  // 合法 → 保留
	})
	if len(fh.events) != 1 {
		t.Fatalf("want 1 event, got %d", len(fh.events))
	}
	props := fh.events[0].Properties
	if _, ok := props["temperature"]; ok {
		t.Fatal("out-of-range temperature must be dropped")
	}
	if _, ok := props["ph"]; ok {
		t.Fatal("out-of-range ph must be dropped")
	}
	if props["dissolved_oxygen"] != 6.8 || props["battery"] != 95 {
		t.Fatalf("in-range fields must survive: %+v", props)
	}
}
