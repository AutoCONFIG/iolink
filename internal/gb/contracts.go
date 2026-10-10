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
	Peer         netip.AddrPort
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
	if r.Status == RegistrationOffline || now.IsZero() || r.ExpiresAt.IsZero() || !now.Before(r.ExpiresAt) || r.LastSeenAt.IsZero() || now.Sub(r.LastSeenAt) >= KeepaliveMissLimit*KeepalivePeriod {
		return RegistrationOffline
	}
	return RegistrationOnline
}

type DeviceStore interface {
	LookupDevice(context.Context, string) (RegisteredDevice, error)
	LookupPeer(context.Context, netip.AddrPort) (Registration, error)
	ApplyRegistration(context.Context, Registration) error
	MarkKeepalive(context.Context, string, netip.AddrPort, time.Time, string) error
}

type NonceRecord struct {
	DeviceID  string
	Realm     string
	Peer      netip.Addr
	URI       string
	ExpiresAt time.Time
}

type NonceStore interface {
	Issue(context.Context, NonceRecord) (string, error)
	Consume(context.Context, string, time.Time) (NonceRecord, error)
}

type Transport interface {
	Send(context.Context, netip.AddrPort, []byte) error
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
	Source        netip.AddrPort
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
	Devices       DeviceStore
	Nonces        NonceStore
	Clock         Clock
	Transport     Transport
	transactionMu sync.Mutex
	transactions  map[string]transactionResult
}

type transactionResult struct {
	result    RegisterResult
	expiresAt time.Time
}

type MessageResult struct {
	Message XMLMessage
	Status  RegistrationStatus
}

func (s *Service) HandleMessage(ctx context.Context, source netip.AddrPort, raw []byte) (MessageResult, error) {
	if s == nil || s.Devices == nil || s.Clock == nil || !source.IsValid() {
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
	if !device.AllowsPeer(source.Addr()) {
		return MessageResult{}, ErrPeerNotAllowed
	}
	registration, err := s.Devices.LookupPeer(ctx, source)
	if err != nil || registration.DeviceID != message.DeviceID {
		return MessageResult{}, ErrUnknownDevice
	}
	if message.CmdType == "Keepalive" {
		if err := s.Devices.MarkKeepalive(ctx, message.DeviceID, source, s.Clock.Now(), message.Status); err != nil {
			return MessageResult{}, ErrUnknownDevice
		}
		status := RegistrationOffline
		if message.Status == "OK" {
			status = RegistrationOnline
		}
		return MessageResult{Message: message, Status: status}, nil
	}
	return MessageResult{Message: message, Status: registration.StatusAt(s.Clock.Now())}, nil
}

func (s *Service) HandleSIPMessage(ctx context.Context, method string, source netip.AddrPort, raw []byte) (MessageResult, error) {
	if method != "MESSAGE" {
		return MessageResult{}, ErrInvalidRequest
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
