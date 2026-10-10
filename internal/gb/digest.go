package gb

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"time"
)

func md5Hex(s string) string { sum := md5.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }

func digestResponse(a DigestAuthorization, method, password string) (string, error) {
	if a.Algorithm != "" && !strings.EqualFold(a.Algorithm, "MD5") {
		return "", ErrDigestInvalid
	}
	if !validText(a.Username, 128) || !validText(a.Realm, 128) || !validText(a.Nonce, 256) || !validText(a.URI, 2048) || !validText(method, 32) || len(password) > 128 {
		return "", ErrDigestInvalid
	}
	if strings.ContainsAny(a.URI, "\r\n") || a.Response == "" {
		return "", ErrDigestInvalid
	}
	ha1 := md5Hex(a.Username + ":" + a.Realm + ":" + password)
	ha2 := md5Hex(method + ":" + a.URI)
	if a.QOP == "" {
		return md5Hex(ha1 + ":" + a.Nonce + ":" + ha2), nil
	}
	if !strings.EqualFold(a.QOP, "auth") || !validText(a.CNonce, 128) || len(a.NC) != 8 {
		return "", ErrDigestInvalid
	}
	for i := range a.NC {
		if !strings.ContainsRune("0123456789abcdefABCDEF", rune(a.NC[i])) {
			return "", ErrDigestInvalid
		}
	}
	return md5Hex(ha1 + ":" + a.Nonce + ":" + a.NC + ":" + a.CNonce + ":auth:" + ha2), nil
}

func (s *Service) Challenge(ctx context.Context, req RegisterRequest) (RegisterResult, error) {
	if s == nil || s.Devices == nil || s.Nonces == nil || s.Clock == nil || !validDeviceID(req.DeviceID) || req.Username != req.DeviceID || !validText(req.Method, 32) || req.Method != "REGISTER" || !req.Source.IsValid() {
		return RegisterResult{}, ErrInvalidRequest
	}
	device, err := s.Devices.LookupDevice(ctx, req.DeviceID)
	if err != nil {
		return RegisterResult{}, ErrUnknownDevice
	}
	if device.DeviceID != req.DeviceID || !device.AllowsPeer(req.Source.Addr()) {
		return RegisterResult{}, ErrPeerNotAllowed
	}
	if !validText(device.Realm, 128) || !validText(req.URI, 2048) {
		return RegisterResult{}, ErrInvalidRequest
	}
	expires := s.Clock.Now().Add(60 * time.Second)
	nonce, err := s.Nonces.Issue(ctx, NonceRecord{DeviceID: req.DeviceID, Realm: device.Realm, Peer: req.Source.Addr(), URI: req.URI, ExpiresAt: expires})
	if err != nil || !validText(nonce, 256) {
		return RegisterResult{}, ErrNonceInvalid
	}
	return RegisterResult{StatusCode: 401, Challenge: &DigestChallenge{Realm: device.Realm, Nonce: nonce, Algorithm: "MD5"}}, nil
}

func (s *Service) HandleRegister(ctx context.Context, req RegisterRequest) (RegisterResult, error) {
	if s == nil || s.Devices == nil || s.Nonces == nil || s.Clock == nil || !validDeviceID(req.DeviceID) || req.Username != req.DeviceID || req.Method != "REGISTER" || !req.Source.IsValid() || !validText(req.URI, 2048) || (req.TransactionID != "" && !validText(req.TransactionID, 128)) {
		return RegisterResult{}, ErrInvalidRequest
	}
	if req.TransactionID != "" {
		if result, ok := s.transaction(req.TransactionID, req.Source, req.DeviceID); ok {
			return result, nil
		}
	}
	device, err := s.Devices.LookupDevice(ctx, req.DeviceID)
	if err != nil {
		return RegisterResult{}, ErrUnknownDevice
	}
	if device.DeviceID != req.DeviceID || !device.AllowsPeer(req.Source.Addr()) {
		return RegisterResult{}, ErrPeerNotAllowed
	}
	if req.Authorization == nil {
		return RegisterResult{}, ErrDigestRequired
	}
	a := *req.Authorization
	if a.Username != req.Username || a.Realm != device.Realm || a.URI != req.URI {
		return RegisterResult{}, ErrDigestInvalid
	}
	record, err := s.Nonces.Consume(ctx, a.Nonce, s.Clock.Now())
	if err != nil {
		if errors.Is(err, ErrNonceReplay) {
			return RegisterResult{}, err
		}
		return RegisterResult{}, ErrNonceInvalid
	}
	if record.DeviceID != req.DeviceID || record.Realm != device.Realm || record.Peer != req.Source.Addr() || record.URI != req.URI || !s.Clock.Now().Before(record.ExpiresAt) {
		return RegisterResult{}, ErrNonceInvalid
	}
	expected, err := digestResponse(a, req.Method, string(device.Password))
	if err != nil || len(a.Response) != len(expected) || subtle.ConstantTimeCompare([]byte(strings.ToLower(a.Response)), []byte(expected)) != 1 {
		return RegisterResult{}, ErrDigestInvalid
	}
	ttl, err := registrationTTL(req.Expires)
	if err != nil {
		return RegisterResult{}, err
	}
	if req.Expires != 0 {
		now := s.Clock.Now()
		reg := Registration{DeviceID: req.DeviceID, Peer: req.Source, RegisteredAt: now, LastSeenAt: now, ExpiresAt: now.Add(ttl), Status: RegistrationOnline}
		if err := s.applyRegistration(ctx, req.Source, req.DeviceID, reg); err != nil {
			return RegisterResult{}, ErrRegistrationConflict
		}
		result := RegisterResult{StatusCode: 200, Registration: reg}
		s.rememberTransaction(req.TransactionID, req.Source, req.DeviceID, result)
		return result, nil
	}
	now := s.Clock.Now()
	reg := Registration{DeviceID: req.DeviceID, Peer: req.Source, RegisteredAt: now, LastSeenAt: now, Status: RegistrationOffline}
	if err := s.applyRegistration(ctx, req.Source, req.DeviceID, reg); err != nil {
		return RegisterResult{}, ErrRegistrationConflict
	}
	result := RegisterResult{StatusCode: 200, Registration: reg}
	s.rememberTransaction(req.TransactionID, req.Source, req.DeviceID, result)
	return result, nil
}

func (s *Service) applyRegistration(ctx context.Context, source netip.AddrPort, deviceID string, registration Registration) error {
	s.registrationMu.Lock()
	defer s.registrationMu.Unlock()
	peerReg, lookupErr := s.Devices.LookupPeer(ctx, source)
	if lookupErr != nil && !errors.Is(lookupErr, ErrPeerNotFound) && !errors.Is(lookupErr, ErrUnknownDevice) {
		return ErrRegistrationConflict
	}
	if lookupErr == nil && peerReg.DeviceID != "" && peerReg.DeviceID != deviceID {
		return ErrRegistrationConflict
	}
	if err := s.Devices.ApplyRegistration(ctx, registration); err != nil {
		return ErrRegistrationConflict
	}
	return nil
}

func (s *Service) transaction(id string, source netip.AddrPort, deviceID string) (RegisterResult, bool) {
	key := id + "|" + source.String() + "|" + deviceID
	now := s.Clock.Now()
	s.transactionMu.Lock()
	defer s.transactionMu.Unlock()
	if s.transactions == nil {
		return RegisterResult{}, false
	}
	for k, item := range s.transactions {
		if !now.Before(item.expiresAt) {
			delete(s.transactions, k)
		}
	}
	item, ok := s.transactions[key]
	return item.result, ok && now.Before(item.expiresAt)
}

func (s *Service) rememberTransaction(id string, source netip.AddrPort, deviceID string, result RegisterResult) {
	if id == "" {
		return
	}
	key := id + "|" + source.String() + "|" + deviceID
	s.transactionMu.Lock()
	defer s.transactionMu.Unlock()
	if s.transactions == nil {
		s.transactions = make(map[string]transactionResult)
	}
	if len(s.transactions) >= 1024 {
		for k := range s.transactions {
			delete(s.transactions, k)
			break
		}
	}
	s.transactions[key] = transactionResult{result: result, expiresAt: s.Clock.Now().Add(60 * time.Second)}
}
