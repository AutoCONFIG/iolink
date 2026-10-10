package gb

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

type testNonceStore struct {
	next     int
	records  map[string]NonceRecord
	consumed map[string]bool
}

func (n *testNonceStore) Issue(_ context.Context, r NonceRecord) (string, error) {
	n.next++
	nonce := "nonce-" + strconv.Itoa(n.next)
	if n.records == nil {
		n.records = map[string]NonceRecord{}
	}
	n.records[nonce] = r
	return nonce, nil
}
func (n *testNonceStore) Consume(_ context.Context, value string, now time.Time) (NonceRecord, error) {
	r, ok := n.records[value]
	if !ok || n.consumed[value] {
		return NonceRecord{}, ErrNonceReplay
	}
	if !now.Before(r.ExpiresAt) {
		n.consumed[value] = true
		return NonceRecord{}, ErrNonceInvalid
	}
	n.consumed[value] = true
	return r, nil
}

type testDeviceStore struct {
	device     RegisteredDevice
	peer       *Registration
	applied    []Registration
	markStatus string
}

func (d *testDeviceStore) LookupDevice(_ context.Context, id string) (RegisteredDevice, error) {
	if id != d.device.DeviceID {
		return RegisteredDevice{}, ErrUnknownDevice
	}
	return d.device, nil
}
func (d *testDeviceStore) LookupPeer(_ context.Context, p netip.AddrPort) (Registration, error) {
	if d.peer != nil && d.peer.Peer == p {
		return *d.peer, nil
	}
	return Registration{}, ErrUnknownDevice
}
func (d *testDeviceStore) ApplyRegistration(_ context.Context, r Registration) error {
	d.applied = append(d.applied, r)
	d.peer = &r
	return nil
}
func (d *testDeviceStore) MarkKeepalive(_ context.Context, id string, p netip.AddrPort, now time.Time, status string) error {
	d.markStatus = status
	state := RegistrationOnline
	if status != "OK" {
		state = RegistrationOffline
	}
	d.peer = &Registration{DeviceID: id, Peer: p, LastSeenAt: now, ExpiresAt: now.Add(time.Minute), Status: state}
	return nil
}

func testService() (*Service, *testDeviceStore, *testNonceStore, *testClock, RegisterRequest) {
	now := time.Date(2026, 10, 11, 2, 0, 0, 0, time.UTC)
	peer := netip.MustParseAddrPort("192.0.2.10:5060")
	device := RegisteredDevice{DeviceID: "34020000002000000001", Realm: "3402000000", Password: []byte("secret"), AllowedPeers: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}}
	store, nonces, clock := &testDeviceStore{device: device}, &testNonceStore{records: map[string]NonceRecord{}, consumed: map[string]bool{}}, &testClock{now: now}
	return &Service{Devices: store, Nonces: nonces, Clock: clock}, store, nonces, clock, RegisterRequest{DeviceID: device.DeviceID, Username: device.DeviceID, Realm: device.Realm, Method: "REGISTER", URI: "sip:" + device.DeviceID + "@3402000000", Source: peer, Expires: 300}
}

func digestFor(req RegisterRequest, nonce, password string, qop bool) DigestAuthorization {
	a := DigestAuthorization{Username: req.Username, Realm: req.Realm, Nonce: nonce, URI: req.URI, Algorithm: "MD5"}
	ha1 := md5Hex(a.Username + ":" + a.Realm + ":" + password)
	ha2 := md5Hex(req.Method + ":" + req.URI)
	if qop {
		a.QOP, a.CNonce, a.NC = "auth", "c0ffee", "00000001"
		a.Response = md5Hex(ha1 + ":" + nonce + ":" + a.NC + ":" + a.CNonce + ":auth:" + ha2)
	} else {
		a.Response = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}
	return a
}

func TestRegisterDigestChallengeAndSuccess(t *testing.T) {
	s, store, nonces, _, req := testService()
	chall, err := s.Challenge(context.Background(), req)
	if err != nil || chall.StatusCode != 401 || chall.Challenge == nil || chall.Challenge.Algorithm != "MD5" {
		t.Fatalf("challenge: %#v %v", chall, err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-1", "secret", false))
	got, err := s.HandleRegister(context.Background(), req)
	if err != nil || got.StatusCode != 200 || got.Registration.Status != RegistrationOnline || len(store.applied) != 1 {
		t.Fatalf("register: %#v %v", got, err)
	}
	if got.Registration.ExpiresAt.Sub(got.Registration.RegisteredAt) != 300*time.Second {
		t.Fatalf("ttl=%s", got.Registration.ExpiresAt.Sub(got.Registration.RegisteredAt))
	}
	_ = nonces
}

func TestRegisterDigestQOPAndReplay(t *testing.T) {
	s, _, _, _, req := testService()
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-1", "secret", true))
	if _, err := s.HandleRegister(context.Background(), req); err != nil {
		t.Fatalf("qop: %v", err)
	}
	if _, err := s.HandleRegister(context.Background(), req); !errors.Is(err, ErrNonceReplay) {
		t.Fatalf("replay=%v", err)
	}
}

func TestRegisterTransactionRetransmitReturnsSameResponse(t *testing.T) {
	s, store, _, _, req := testService()
	req.TransactionID = "branch-z9hG4bK-1"
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-1", "secret", false))
	first, err := s.HandleRegister(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.HandleRegister(context.Background(), req)
	if err != nil || second.StatusCode != first.StatusCode || !second.Registration.ExpiresAt.Equal(first.Registration.ExpiresAt) || len(store.applied) != 1 {
		t.Fatalf("retransmit first=%#v second=%#v err=%v applied=%d", first, second, err, len(store.applied))
	}
}

func TestRegisterRejectsUnknownPeerDeviceAndDigestFields(t *testing.T) {
	s, _, _, _, req := testService()
	bad := req
	bad.Source = netip.MustParseAddrPort("198.51.100.5:5060")
	if _, err := s.Challenge(context.Background(), bad); !errors.Is(err, ErrPeerNotAllowed) {
		t.Fatalf("peer=%v", err)
	}
	unknown := req
	unknown.DeviceID = "34020000002000000002"
	unknown.Username = unknown.DeviceID
	if _, err := s.Challenge(context.Background(), unknown); !errors.Is(err, ErrUnknownDevice) {
		t.Fatalf("unknown=%v", err)
	}
	if _, err := s.HandleRegister(context.Background(), req); !errors.Is(err, ErrDigestRequired) {
		t.Fatalf("missing digest=%v", err)
	}
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	a := digestFor(req, "nonce-1", "secret", false)
	a.URI = "sip:other@bad"
	req.Authorization = ptrDigest(a)
	if _, err := s.HandleRegister(context.Background(), req); !errors.Is(err, ErrDigestInvalid) {
		t.Fatalf("uri=%v", err)
	}
}

func TestRegisterExpiresBoundariesAndConflict(t *testing.T) {
	s, store, _, clock, req := testService()
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-1", "secret", false))
	req.Expires = 59
	if _, err := s.HandleRegister(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("short ttl=%v", err)
	}
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-2", "secret", false))
	req.Expires = 0
	if _, err := s.HandleRegister(context.Background(), req); err != nil {
		t.Fatalf("unregister=%v", err)
	}
	if store.applied[len(store.applied)-1].Status != RegistrationOffline {
		t.Fatalf("unregister status=%s", store.applied[len(store.applied)-1].Status)
	}
	clock.now = clock.now.Add(time.Second)
	peerReg := Registration{DeviceID: "34020000002000000009", Peer: req.Source}
	store.peer = &peerReg
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-3", "secret", false))
	req.Expires = 60
	if _, err := s.HandleRegister(context.Background(), req); !errors.Is(err, ErrRegistrationConflict) {
		t.Fatalf("conflict=%v", err)
	}
}

func TestRegistrationStatusExpiresAndKeepaliveThreshold(t *testing.T) {
	now := time.Date(2026, 10, 11, 2, 0, 0, 0, time.UTC)
	base := Registration{LastSeenAt: now, ExpiresAt: now.Add(time.Hour)}
	if base.StatusAt(now) != RegistrationOnline {
		t.Fatal("fresh registration offline")
	}
	if base.StatusAt(now.Add(KeepaliveMissLimit*KeepalivePeriod)) != RegistrationOffline {
		t.Fatal("missed keepalives online")
	}
	base.ExpiresAt = now
	if base.StatusAt(now) != RegistrationOffline {
		t.Fatal("expired registration online")
	}
	base.ExpiresAt = now.Add(time.Hour)
	base.Status = RegistrationOffline
	if base.StatusAt(now) != RegistrationOffline {
		t.Fatal("explicit offline status online")
	}
}

func TestHandleKeepaliveRequiresRegisteredPeer(t *testing.T) {
	s, store, _, _, req := testService()
	if _, err := s.Challenge(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Authorization = ptrDigest(digestFor(req, "nonce-1", "secret", false))
	if _, err := s.HandleRegister(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`<Notify><CmdType>Keepalive</CmdType><SN>11</SN><DeviceID>` + req.DeviceID + `</DeviceID><Status>OK</Status></Notify>`)
	got, err := s.HandleMessage(context.Background(), req.Source, raw)
	if err != nil || got.Status != RegistrationOnline || store.markStatus != "OK" {
		t.Fatalf("keepalive=%#v %v", got, err)
	}
	bad := req.Source
	bad = netip.AddrPortFrom(netip.MustParseAddr("192.0.2.11"), bad.Port())
	if _, err := s.HandleMessage(context.Background(), bad, raw); !errors.Is(err, ErrUnknownDevice) {
		t.Fatalf("unregistered peer=%v", err)
	}
}

func TestMemoryNonceStoreExpiresAndRejectsReplay(t *testing.T) {
	nonces := NewMemoryNonceStore()
	now := time.Date(2026, 10, 11, 2, 0, 0, 0, time.UTC)
	nonce, err := nonces.Issue(context.Background(), NonceRecord{DeviceID: "34020000002000000001", Peer: netip.MustParseAddr("192.0.2.10"), ExpiresAt: now.Add(time.Minute)})
	if err != nil || len(nonce) < 20 {
		t.Fatalf("nonce=%q err=%v", nonce, err)
	}
	if _, err := nonces.Consume(context.Background(), nonce, now); err != nil {
		t.Fatal(err)
	}
	if _, err := nonces.Consume(context.Background(), nonce, now); !errors.Is(err, ErrNonceReplay) {
		t.Fatalf("replay=%v", err)
	}
	expired, err := nonces.Issue(context.Background(), NonceRecord{DeviceID: "34020000002000000001", Peer: netip.MustParseAddr("192.0.2.10"), ExpiresAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nonces.Consume(context.Background(), expired, now); !errors.Is(err, ErrNonceInvalid) {
		t.Fatalf("expired=%v", err)
	}
}

func TestMemoryNonceStoreIsBounded(t *testing.T) {
	nonces := NewMemoryNonceStore()
	expires := time.Now().Add(time.Hour)
	for i := 0; i < MaxMemoryNonces+1; i++ {
		if _, err := nonces.Issue(context.Background(), NonceRecord{DeviceID: "34020000002000000001", ExpiresAt: expires}); err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
	}
	nonces.mu.Lock()
	got := len(nonces.entries)
	nonces.mu.Unlock()
	if got > MaxMemoryNonces {
		t.Fatalf("nonce store grew to %d", got)
	}
}

func TestDigestVectorAndValidation(t *testing.T) {
	a := DigestAuthorization{Username: "Mufasa", Realm: "testrealm@host.com", Nonce: "dcd98b7102dd2f0e8b11d0f600bfb0c093", URI: "/dir/index.html", Response: "6629fae49393a05397450978507c4ef1", QOP: "auth", NC: "00000001", CNonce: "0a4f113b"}
	got, err := digestResponse(a, "GET", "Circle Of Life")
	if err != nil || got != a.Response {
		t.Fatalf("vector got=%s err=%v", got, err)
	}
	a.QOP = "auth-int"
	if _, err := digestResponse(a, "GET", "Circle Of Life"); !errors.Is(err, ErrDigestInvalid) {
		t.Fatalf("qop=%v", err)
	}
	if _, err := digestResponse(DigestAuthorization{Username: "u", Realm: "r", Nonce: "n", URI: "/", Response: "x", Algorithm: "SHA-256"}, "GET", "p"); !errors.Is(err, ErrDigestInvalid) {
		t.Fatal("algorithm accepted")
	}
	sum := md5.Sum([]byte("x"))
	if hex.EncodeToString(sum[:]) == "" {
		t.Fatal("unreachable")
	}
}

func ptrDigest(a DigestAuthorization) *DigestAuthorization { return &a }

func TestMessageXMLKeepaliveCatalogAndBounds(t *testing.T) {
	device := "34020000002000000001"
	keepalive := `<Notify><CmdType>Keepalive</CmdType><SN>9</SN><DeviceID>` + device + `</DeviceID><Status>OK</Status></Notify>`
	msg, err := ParseMessageXML([]byte(keepalive), device)
	if err != nil || msg.CmdType != "Keepalive" || msg.Status != "OK" {
		t.Fatalf("keepalive=%#v %v", msg, err)
	}
	catalog := `<Response><CmdType>Catalog</CmdType><SN>10</SN><DeviceID>` + device + `</DeviceID><SumNum>1</SumNum><DeviceList Num="1"><Item><DeviceID>34020000001310000101</DeviceID><Name>front</Name><Status>ON</Status></Item></DeviceList></Response>`
	msg, err = ParseMessageXML([]byte(catalog), device)
	if err != nil || msg.SumNum != 1 || len(msg.Items) != 1 {
		t.Fatalf("catalog=%#v %v", msg, err)
	}
	page := strings.Replace(catalog, "<SumNum>1</SumNum>", "<SumNum>2</SumNum>", 1)
	msg, err = ParseMessageXML([]byte(page), device)
	if err != nil || msg.SumNum != 2 || len(msg.Items) != 1 {
		t.Fatalf("catalog page=%#v %v", msg, err)
	}
	for name, raw := range map[string]string{"dtd": `<!DOCTYPE Notify [<!ENTITY x "y">]><Notify/>`, "entity": `<Notify><CmdType>Keepalive</CmdType><SN>1</SN><DeviceID>` + device + `</DeviceID><Status>&x;</Status></Notify>`, "sum": `<Response><CmdType>Catalog</CmdType><SN>1</SN><DeviceID>` + device + `</DeviceID><SumNum>1</SumNum><DeviceList Num="0"/></Response>`} {
		if _, err := ParseMessageXML([]byte(raw), device); !errors.Is(err, ErrXMLInvalid) {
			t.Fatalf("%s err=%v", name, err)
		}
	}
	if _, err := ParseMessageXML([]byte(strings.Repeat("x", MaxXMLSize+1)), device); !errors.Is(err, ErrXMLTooLarge) {
		t.Fatalf("oversize=%v", err)
	}
	if _, err := ParseMessageXML([]byte(keepalive), "34020000002000000002"); !errors.Is(err, ErrXMLInvalid) {
		t.Fatalf("device mismatch=%v", err)
	}
	if _, err := ParseMessageXML(append([]byte(keepalive), []byte(`<Notify/>`)...), device); !errors.Is(err, ErrXMLInvalid) {
		t.Fatalf("trailing root=%v", err)
	}
	if _, err := ParseMessageXML(append([]byte(keepalive), []byte(" trailing")...), device); !errors.Is(err, ErrXMLInvalid) {
		t.Fatalf("trailing text=%v", err)
	}
}

func TestMessageXMLRejectsDepthAndCatalogCount(t *testing.T) {
	device := "34020000002000000001"
	deep := strings.Repeat("<x>", MaxXMLDepth) + "<Notify><CmdType>Keepalive</CmdType><SN>1</SN><DeviceID>" + device + "</DeviceID><Status>OK</Status></Notify>" + strings.Repeat("</x>", MaxXMLDepth)
	if _, err := ParseMessageXML([]byte(deep), device); !errors.Is(err, ErrXMLDepth) && !errors.Is(err, ErrXMLInvalid) {
		t.Fatalf("depth=%v", err)
	}
	if _, err := ParseMessageXML([]byte(`<Response><CmdType>Catalog</CmdType><SN>1</SN><DeviceID>`+device+`</DeviceID><SumNum>1001</SumNum><DeviceList Num="1001"/></Response>`), device); !errors.Is(err, ErrXMLInvalid) {
		t.Fatalf("count=%v", err)
	}
}
