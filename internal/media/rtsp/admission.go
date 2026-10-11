package rtsp

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const DefaultPort uint16 = 554

var (
	ErrInvalidConfig     = errors.New("invalid rtsp admission configuration")
	ErrInvalidURI        = errors.New("invalid rtsp source uri")
	ErrPortNotAllowed    = errors.New("rtsp source port not allowed")
	ErrResolutionFailed  = errors.New("rtsp source resolution failed")
	ErrAddressNotAllowed = errors.New("rtsp source address not allowed")
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type Config struct {
	Resolver     Resolver
	AllowedCIDRs []netip.Prefix
	AllowedPorts []uint16
}

type Admission struct {
	resolver Resolver
	cidrs    []netip.Prefix
	ports    map[uint16]struct{}
}

type Endpoint struct {
	Address netip.Addr
	Port    uint16
	Path    string
}

func New(config Config) (*Admission, error) {
	if config.Resolver == nil || len(config.AllowedCIDRs) == 0 || len(config.AllowedPorts) == 0 {
		return nil, ErrInvalidConfig
	}
	cidrs := make([]netip.Prefix, 0, len(config.AllowedCIDRs))
	for _, cidr := range config.AllowedCIDRs {
		if !cidr.IsValid() {
			return nil, ErrInvalidConfig
		}
		cidrs = append(cidrs, cidr.Masked())
	}
	ports := make(map[uint16]struct{}, len(config.AllowedPorts))
	for _, port := range config.AllowedPorts {
		if port == 0 {
			return nil, ErrInvalidConfig
		}
		ports[port] = struct{}{}
	}
	return &Admission{resolver: config.Resolver, cidrs: cidrs, ports: ports}, nil
}

func (a *Admission) Pin(ctx context.Context, raw string) (Endpoint, error) {
	if a == nil || ctx == nil {
		return Endpoint{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return Endpoint{}, err
	}
	u, host, port, err := parseURI(raw)
	if err != nil {
		return Endpoint{}, err
	}
	if _, ok := a.ports[port]; !ok {
		return Endpoint{}, ErrPortNotAllowed
	}
	addresses, err := a.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Endpoint{}, ctxErr
		}
		return Endpoint{}, ErrResolutionFailed
	}
	if len(addresses) == 0 {
		return Endpoint{}, ErrResolutionFailed
	}
	addresses = uniqueAddresses(addresses)
	for _, address := range addresses {
		if !safeAddress(address) || !a.allowed(address) {
			return Endpoint{}, ErrAddressNotAllowed
		}
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].Compare(addresses[j]) < 0 })
	if err := ctx.Err(); err != nil {
		return Endpoint{}, err
	}
	return Endpoint{Address: addresses[0], Port: port, Path: u.EscapedPath()}, nil
}

func (e Endpoint) AddrPort() netip.AddrPort {
	return netip.AddrPortFrom(e.Address, e.Port)
}

func (e Endpoint) URI() string {
	if !e.Address.IsValid() || e.Port == 0 {
		return ""
	}
	return "rtsp://" + e.AddrPort().String() + e.Path
}

func parseURI(raw string) (*url.URL, string, uint16, error) {
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsAny(raw, "?#\\") || strings.ContainsFunc(raw, unicode.IsControl) {
		return nil, "", 0, ErrInvalidURI
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "rtsp" || u.Opaque != "" || u.User != nil || u.Host == "" {
		return nil, "", 0, ErrInvalidURI
	}
	host := u.Hostname()
	if host == "" || strings.ContainsAny(host, "% \t\r\n") {
		return nil, "", 0, ErrInvalidURI
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, "", 0, ErrInvalidURI
	}
	port := DefaultPort
	if rawPort := u.Port(); rawPort != "" {
		parsed, parseErr := strconv.ParseUint(rawPort, 10, 16)
		if parseErr != nil || parsed == 0 {
			return nil, "", 0, ErrInvalidURI
		}
		port = uint16(parsed)
	}
	if err := validatePath(u.EscapedPath()); err != nil {
		return nil, "", 0, err
	}
	return u, host, port, nil
}

func validatePath(path string) error {
	decoded, err := url.PathUnescape(path)
	if err != nil || strings.ContainsFunc(decoded, unicode.IsControl) || strings.Contains(decoded, "\\") {
		return ErrInvalidURI
	}
	if strings.Contains(strings.ToLower(decoded), "%2e") || strings.Contains(strings.ToLower(decoded), "%5c") {
		return ErrInvalidURI
	}
	for segment := range strings.SplitSeq(decoded, "/") {
		if segment == ".." {
			return ErrInvalidURI
		}
	}
	return nil
}

func uniqueAddresses(addresses []netip.Addr) []netip.Addr {
	seen := make(map[netip.Addr]struct{}, len(addresses))
	unique := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		unique = append(unique, address)
	}
	return unique
}

func safeAddress(address netip.Addr) bool {
	if !address.IsValid() || address.IsUnspecified() || address.IsMulticast() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
		return false
	}
	for _, metadata := range []netip.Prefix{
		netip.MustParsePrefix("169.254.169.254/32"),
		netip.MustParsePrefix("100.100.100.200/32"),
		netip.MustParsePrefix("fd00:ec2::254/128"),
	} {
		if metadata.Contains(address) {
			return false
		}
	}
	return true
}

func (a *Admission) allowed(address netip.Addr) bool {
	for _, cidr := range a.cidrs {
		if cidr.Contains(address) {
			return true
		}
	}
	return false
}
