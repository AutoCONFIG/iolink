package gb

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"
)

var (
	ErrInvalidRequest       = errors.New("invalid GB request")
	ErrUnknownDevice        = errors.New("unknown GB device")
	ErrPeerNotAllowed       = errors.New("GB peer not allowed")
	ErrPeerConflict         = errors.New("GB peer already registered")
	ErrPeerNotFound         = errors.New("GB peer registration not found")
	ErrDigestRequired       = errors.New("GB digest required")
	ErrDigestInvalid        = errors.New("invalid GB digest")
	ErrNonceInvalid         = errors.New("invalid GB nonce")
	ErrNonceReplay          = errors.New("GB nonce replay")
	ErrRegistrationConflict = errors.New("GB registration conflict")
	ErrXMLInvalid           = errors.New("invalid GB XML")
	ErrXMLTooLarge          = errors.New("GB XML too large")
	ErrXMLDepth             = errors.New("GB XML depth exceeded")
)

const (
	MaxSIPMessageSize  = 64 * 1024
	MaxXMLSize         = 64 * 1024
	MaxXMLDepth        = 16
	MaxCatalogItems    = 1000
	KeepalivePeriod    = 60 * time.Second
	KeepaliveMissLimit = 3
	MinRegistrationTTL = 60 * time.Second
	MaxRegistrationTTL = time.Hour
)

type Clock interface{ Now() time.Time }

type TransportProtocol string

const (
	TransportUDP TransportProtocol = "udp"
	TransportTCP TransportProtocol = "tcp"
)

func (p TransportProtocol) valid() bool {
	return p == TransportUDP || p == TransportTCP
}

type PeerIdentity struct {
	Addr      netip.AddrPort
	Transport TransportProtocol
}

func NewPeerIdentity(addr netip.AddrPort, transport TransportProtocol) (PeerIdentity, error) {
	identity := PeerIdentity{Addr: addr, Transport: transport}
	if !identity.Valid() {
		return PeerIdentity{}, ErrInvalidRequest
	}
	return identity, nil
}

func (p PeerIdentity) Valid() bool {
	return p.Addr.IsValid() && p.Transport.valid()
}

type RegisteredDevice struct {
	DeviceID     string
	Realm        string
	Password     []byte
	AllowedPeers []netip.Prefix
}

func (d RegisteredDevice) AllowsPeer(peer netip.Addr) bool {
	if !peer.IsValid() || len(d.AllowedPeers) == 0 {
		return false
	}
	for _, prefix := range d.AllowedPeers {
		if prefix.Contains(peer) {
			return true
		}
	}
	return false
}

type Registration struct {
	DeviceID     string
	Peer         PeerIdentity
	RegisteredAt time.Time
	LastSeenAt   time.Time
	ExpiresAt    time.Time
	Status       RegistrationStatus
}

type RegistrationStatus string

const (
	RegistrationOnline  RegistrationStatus = "online"
	RegistrationOffline RegistrationStatus = "offline"
)

func (r Registration) StatusAt(now time.Time) RegistrationStatus {
	if r.Status != RegistrationOnline || now.IsZero() || r.ExpiresAt.IsZero() || !now.Before(r.ExpiresAt) || r.LastSeenAt.IsZero() || now.Sub(r.LastSeenAt) >= KeepaliveMissLimit*KeepalivePeriod {
		return RegistrationOffline
	}
	return RegistrationOnline
}

type DeviceStore interface {
	LookupDevice(context.Context, string) (RegisteredDevice, error)
	LookupPeer(context.Context, PeerIdentity) (Registration, error)
	ApplyRegistration(context.Context, Registration) error
	MarkKeepalive(context.Context, string, PeerIdentity, time.Time, string) error
}

type NonceRecord struct {
	DeviceID  string
	Realm     string
	Peer      PeerIdentity
	URI       string
	ExpiresAt time.Time
}

type NonceStore interface {
	Issue(context.Context, NonceRecord) (string, error)
	Consume(context.Context, string, time.Time) (NonceRecord, error)
}

type Transport interface {
	Send(context.Context, PeerIdentity, []byte) error
}

type DigestAuthorization struct {
	Username  string
	Realm     string
	Nonce     string
	URI       string
	Response  string
	Algorithm string
	QOP       string
	CNonce    string
	NC        string
}

type RegisterRequest struct {
	DeviceID      string
	Username      string
	Realm         string
	Method        string
	URI           string
	Source        PeerIdentity
	Expires       int
	Authorization *DigestAuthorization
	TransactionID string
}

type DigestChallenge struct {
	Realm     string
	Nonce     string
	Algorithm string
}

type RegisterResult struct {
	StatusCode   int
	Challenge    *DigestChallenge
	Registration Registration
}

type Service struct {
	Devices        DeviceStore
	Nonces         NonceStore
	Clock          Clock
	Transport      Transport
	transactionMu  sync.Mutex
	registrationMu sync.Mutex
	transactions   map[string]transactionResult
}

type transactionResult struct {
	result    RegisterResult
	expiresAt time.Time
}

type MessageResult struct {
	Message XMLMessage
	Status  RegistrationStatus
}

func (s *Service) HandleMessage(ctx context.Context, source PeerIdentity, raw []byte) (MessageResult, error) {
	if s == nil || s.Devices == nil || s.Clock == nil || !source.Valid() {
		return MessageResult{}, ErrInvalidRequest
	}
	message, err := ParseMessageXML(raw, "")
	if err != nil {
		return MessageResult{}, err
	}
	device, err := s.Devices.LookupDevice(ctx, message.DeviceID)
	if err != nil {
		return MessageResult{}, ErrUnknownDevice
	}
	if !device.AllowsPeer(source.Addr.Addr()) {
		return MessageResult{}, ErrPeerNotAllowed
	}
	registration, err := s.Devices.LookupPeer(ctx, source)
	if err != nil || registration.DeviceID != message.DeviceID || registration.Peer != source {
		return MessageResult{}, ErrUnknownDevice
	}
	now := s.Clock.Now()
	if registration.StatusAt(now) != RegistrationOnline {
		return MessageResult{}, ErrUnknownDevice
	}
	if message.CmdType == "Keepalive" {
		if err := s.Devices.MarkKeepalive(ctx, message.DeviceID, source, now, message.Status); err != nil {
			return MessageResult{}, ErrUnknownDevice
		}
		status := RegistrationOffline
		if message.Status == "OK" {
			status = RegistrationOnline
		}
		return MessageResult{Message: message, Status: status}, nil
	}
	return MessageResult{Message: message, Status: registration.StatusAt(now)}, nil
}

func (s *Service) HandleSIPMessage(ctx context.Context, method string, source PeerIdentity, raw []byte) (MessageResult, error) {
	if method != "MESSAGE" {
		return MessageResult{}, ErrInvalidRequest
	}
	if len(raw) > MaxSIPMessageSize {
		return MessageResult{}, ErrXMLTooLarge
	}
	return s.HandleMessage(ctx, source, raw)
}

func validDeviceID(id string) bool {
	if len(id) != 20 {
		return false
	}
	for i := range id {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	return true
}

func validText(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for i := range s {
		if s[i] < 0x20 || s[i] == 0x7f {
			return false
		}
	}
	return true
}

func registrationTTL(seconds int) (time.Duration, error) {
	if seconds == 0 {
		return 0, nil
	}
	if seconds < 0 || seconds > int(MaxRegistrationTTL/time.Second) {
		return 0, ErrInvalidRequest
	}
	ttl := time.Duration(seconds) * time.Second
	if ttl < MinRegistrationTTL || ttl > MaxRegistrationTTL {
		return 0, ErrInvalidRequest
	}
	return ttl, nil
}
