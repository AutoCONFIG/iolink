package gbpg_test

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/gb"
	"git.hyhy.fun/rsplab/iolink/internal/gbpg"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	deviceID = "34020000001320001701"
	peerIP   = "192.0.2.10"
)

type fixture struct {
	p      *pgxpool.Pool
	store  *gbpg.Store
	cipher domain.VideoCredentialCipher
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	p := testdb.New(t)
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	c, err := videocredential.New([]byte("gbpg-test-root-key-0123456789012345"))
	if err != nil {
		t.Fatal(err)
	}
	binding, err := domain.NewVideoCredentialBinding(domain.GBDeviceCredential, 1701, 1701, 1)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := c.Seal(t.Context(), binding, []byte("device-secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(t.Context(), `INSERT INTO tenants(id,name) VALUES (1701,'gbpg')`)
	if err == nil {
		_, err = p.Exec(t.Context(), `INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher,realm,allowed_peer_cidrs)
 VALUES (1701,1701,$1,'GBPG',$2,'3402000000',ARRAY['192.0.2.0/24'::cidr,'2001:db8::/32'::cidr])`, deviceID, sealed)
	}
	if err != nil {
		t.Fatal(err)
	}
	return fixture{p: p, store: gbpg.New(p, c), cipher: c}
}

func peer(transport gb.TransportProtocol) gb.PeerIdentity {
	identity, err := gb.NewPeerIdentity(netip.MustParseAddrPort(peerIP+":5060"), transport)
	if err != nil {
		panic(err)
	}
	return identity
}

func registration(id string, p gb.PeerIdentity, now time.Time) gb.Registration {
	return gb.Registration{DeviceID: id, Peer: p, RegisteredAt: now, LastSeenAt: now, ExpiresAt: now.Add(5 * time.Minute), Status: gb.RegistrationOnline}
}

func TestLookupDeviceDecryptsBoundCredentialAndRejectsEmptyPolicy(t *testing.T) {
	f := newFixture(t)
	got, err := f.store.LookupDevice(t.Context(), deviceID)
	if err != nil || got.DeviceID != deviceID || got.Realm != "3402000000" || string(got.Password) != "device-secret" || !got.AllowsPeer(netip.MustParseAddr(peerIP)) {
		t.Fatalf("lookup=%+v err=%v", got, err)
	}
	var stored []byte
	if err := f.p.QueryRow(t.Context(), `SELECT credential_cipher FROM video_gb_devices WHERE device_id=$1`, deviceID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) == "device-secret" {
		t.Fatal("credential was stored in plaintext")
	}
	if _, err := f.store.LookupDevice(t.Context(), "unknown"); !errors.Is(err, gb.ErrUnknownDevice) {
		t.Fatalf("unknown device err=%v", err)
	}
	if _, err := f.p.Exec(t.Context(), `UPDATE video_gb_devices SET allowed_peer_cidrs='{}' WHERE device_id=$1`, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.LookupDevice(t.Context(), deviceID); !errors.Is(err, gb.ErrUnknownDevice) {
		t.Fatalf("empty peer policy err=%v", err)
	}
}

func TestRegistrationBindsTransportAndKeepaliveAndUnregisters(t *testing.T) {
	f := newFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	udp := peer(gb.TransportUDP)
	if err := f.store.ApplyRegistration(t.Context(), registration(deviceID, udp, now)); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.LookupPeer(t.Context(), udp)
	if err != nil || got.DeviceID != deviceID || got.Peer.Transport != gb.TransportUDP {
		t.Fatalf("lookup=%+v err=%v", got, err)
	}
	tcp := peer(gb.TransportTCP)
	if err := f.store.ApplyRegistration(t.Context(), registration(deviceID, tcp, now)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.LookupPeer(t.Context(), udp); !errors.Is(err, gb.ErrPeerNotFound) {
		t.Fatalf("old transport remains registered: %v", err)
	}
	if _, err := f.store.LookupPeer(t.Context(), tcp); err != nil {
		t.Fatal(err)
	}
	seen := now.Add(time.Second)
	if err := f.store.MarkKeepalive(t.Context(), deviceID, tcp, seen, "OK"); err != nil {
		t.Fatal(err)
	}
	got, err = f.store.LookupPeer(t.Context(), tcp)
	if err != nil || !got.LastSeenAt.Equal(seen) {
		t.Fatalf("keepalive=%+v err=%v", got, err)
	}
	if err := f.store.ApplyRegistration(t.Context(), gb.Registration{DeviceID: deviceID, Peer: tcp, RegisteredAt: seen, Status: gb.RegistrationOffline}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.LookupPeer(t.Context(), tcp); !errors.Is(err, gb.ErrPeerNotFound) {
		t.Fatalf("unregistered peer lookup=%v", err)
	}
}

func TestRegistrationRejectsDisabledExpiredAndOutOfPolicyPeers(t *testing.T) {
	f := newFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	outside, err := gb.NewPeerIdentity(netip.MustParseAddrPort("198.51.100.10:5060"), gb.TransportUDP)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.ApplyRegistration(t.Context(), registration(deviceID, outside, now)); !errors.Is(err, gb.ErrPeerNotAllowed) {
		t.Fatalf("outside peer err=%v", err)
	}
	if err := f.store.ApplyRegistration(t.Context(), registration(deviceID, peer(gb.TransportUDP), now.Add(-2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if _, err := f.p.Exec(t.Context(), `UPDATE video_gb_devices SET registered_until=clock_timestamp()-interval '1 second' WHERE device_id=$1`, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.LookupPeer(t.Context(), peer(gb.TransportUDP)); !errors.Is(err, gb.ErrPeerNotFound) {
		t.Fatalf("expired peer err=%v", err)
	}
	if _, err := f.p.Exec(t.Context(), `UPDATE video_gb_devices SET enabled=false WHERE device_id=$1`, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.LookupDevice(t.Context(), deviceID); !errors.Is(err, gb.ErrUnknownDevice) {
		t.Fatalf("disabled device err=%v", err)
	}
}

func TestSameAddressPortDifferentTransportAndUniqueConflict(t *testing.T) {
	f := newFixture(t)
	if _, err := f.p.Exec(t.Context(), `INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher,realm,allowed_peer_cidrs)
 VALUES (1702,1701,'34020000001320001702','GBPG 2',decode(repeat('aa',29),'hex'),'3402000000',ARRAY['192.0.2.0/24'::cidr])`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := f.store.ApplyRegistration(t.Context(), registration(deviceID, peer(gb.TransportUDP), now)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ApplyRegistration(t.Context(), registration("34020000001320001702", peer(gb.TransportTCP), now)); err != nil {
		t.Fatal(err)
	}
	const workers = 2
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for _, id := range []string{deviceID, "34020000001320001702"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			errs <- f.store.ApplyRegistration(context.Background(), registration(id, peer(gb.TransportUDP), now.Add(time.Second)))
		}(id)
	}
	wg.Wait()
	close(errs)
	var ok, conflicts int
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, gb.ErrPeerConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent registration err=%v", err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("concurrent registration outcomes ok=%d conflicts=%d", ok, conflicts)
	}
}
