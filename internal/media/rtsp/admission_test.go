package rtsp_test

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/media/rtsp"
)

type resolver struct {
	mu      sync.Mutex
	answers [][]netip.Addr
	err     error
	calls   []string
}

func (r *resolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, network+":"+host)
	if r.err != nil {
		return nil, r.err
	}
	if len(r.answers) == 0 {
		return nil, nil
	}
	answer := r.answers[0]
	r.answers = r.answers[1:]
	return append([]netip.Addr(nil), answer...), nil
}

func admission(t *testing.T, r *resolver, cidrs ...string) *rtsp.Admission {
	t.Helper()
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, raw := range cidrs {
		prefixes = append(prefixes, netip.MustParsePrefix(raw))
	}
	got, err := rtsp.New(rtsp.Config{Resolver: r, AllowedCIDRs: prefixes, AllowedPorts: []uint16{554, 8554}})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestNewRejectsMissingAdmissionBoundaries(t *testing.T) {
	r := &resolver{}
	for name, config := range map[string]rtsp.Config{
		"resolver":  {AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}, AllowedPorts: []uint16{554}},
		"cidr":      {Resolver: r, AllowedPorts: []uint16{554}},
		"port":      {Resolver: r, AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}},
		"zero port": {Resolver: r, AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}, AllowedPorts: []uint16{0}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := rtsp.New(config); !errors.Is(err, rtsp.ErrInvalidConfig) {
				t.Fatalf("error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestPinResolvesAllAddressesAndReturnsNumericPath(t *testing.T) {
	r := &resolver{answers: [][]netip.Addr{{netip.MustParseAddr("2001:db8::20"), netip.MustParseAddr("192.0.2.20"), netip.MustParseAddr("192.0.2.20")}}}
	a := admission(t, r, "192.0.2.0/24", "2001:db8::/32")
	got, err := a.Pin(t.Context(), "rtsp://camera.example:8554/live/%E6%B5%81%20one")
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != netip.MustParseAddr("192.0.2.20") || got.Port != 8554 || got.Path != "/live/%E6%B5%81%20one" {
		t.Fatalf("endpoint = %+v", got)
	}
	if got.URI() != "rtsp://192.0.2.20:8554/live/%E6%B5%81%20one" {
		t.Fatalf("numeric URI = %q", got.URI())
	}
	if len(r.calls) != 1 || r.calls[0] != "ip:camera.example" {
		t.Fatalf("resolver calls = %v", r.calls)
	}
}

func TestPinRejectsUnsafeSyntaxAndPortsBeforeResolution(t *testing.T) {
	r := &resolver{}
	a := admission(t, r, "192.0.2.0/24")
	for _, raw := range []string{
		"rtsp://user:password@camera.example/live",
		"rtsp://camera.example/live?x=1",
		"rtsp://camera.example/live#frag",
		"rtsp://camera.example/live/../secret",
		"rtsp://camera.example/live/%2e%2e/secret",
		"rtsp://camera.example/live/%252e%252e/secret",
		"rtsp://camera.example:9000/live",
	} {
		if _, err := a.Pin(t.Context(), raw); !errors.Is(err, rtsp.ErrInvalidURI) && !errors.Is(err, rtsp.ErrPortNotAllowed) {
			t.Errorf("%q error = %v", raw, err)
		}
	}
	if len(r.calls) != 0 {
		t.Fatalf("resolver called for invalid input: %v", r.calls)
	}
}

func TestPinRejectsEveryUnsafeOrDisallowedAddress(t *testing.T) {
	unsafe := []netip.Addr{
		netip.MustParseAddr("0.0.0.0"),
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("224.0.0.1"),
		netip.MustParseAddr("169.254.169.254"),
		netip.MustParseAddr("100.100.100.200"),
	}
	for _, address := range unsafe {
		r := &resolver{answers: [][]netip.Addr{{address}}}
		a := admission(t, r, "0.0.0.0/0", "::/0")
		if _, err := a.Pin(t.Context(), "rtsp://camera.example/live"); !errors.Is(err, rtsp.ErrAddressNotAllowed) {
			t.Errorf("address %s error = %v", address, err)
		}
	}
	r := &resolver{answers: [][]netip.Addr{{netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.7")}}}
	a := admission(t, r, "198.51.100.0/24")
	if _, err := a.Pin(t.Context(), "rtsp://camera.example/live"); !errors.Is(err, rtsp.ErrAddressNotAllowed) {
		t.Fatalf("mixed answer error = %v", err)
	}
}

func TestPinReResolvesOnEveryCallAndContextCancellationFailsClosed(t *testing.T) {
	r := &resolver{answers: [][]netip.Addr{{netip.MustParseAddr("192.0.2.10")}, {netip.MustParseAddr("192.0.2.11")}}}
	a := admission(t, r, "192.0.2.0/24")
	first, err := a.Pin(t.Context(), "rtsp://camera.example/live")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Pin(t.Context(), "rtsp://camera.example/live")
	if err != nil {
		t.Fatal(err)
	}
	if first.Address == second.Address || len(r.calls) != 2 {
		t.Fatalf("pins = %s, %s calls=%v", first.Address, second.Address, r.calls)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := a.Pin(canceled, "rtsp://camera.example/live"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled pin error = %v", err)
	}
}

func TestPinConcurrentCallsRemainRaceFree(t *testing.T) {
	r := &resolver{answers: make([][]netip.Addr, 64)}
	for i := range r.answers {
		r.answers[i] = []netip.Addr{netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})}
	}
	a := admission(t, r, "192.0.2.0/24")
	errs := make(chan error, 64)
	for range 64 {
		go func() {
			_, err := a.Pin(context.Background(), "rtsp://camera.example/live")
			errs <- err
		}()
	}
	for range 64 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}
