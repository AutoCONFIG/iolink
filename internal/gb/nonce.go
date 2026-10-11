package gb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type MemoryNonceStore struct {
	mu      sync.Mutex
	entries map[string]NonceRecord
	used    map[string]time.Time
}

const MaxMemoryNonces = 4096

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{entries: make(map[string]NonceRecord), used: make(map[string]time.Time)}
}

func (s *MemoryNonceStore) Issue(_ context.Context, record NonceRecord) (string, error) {
	if s == nil || !record.Peer.Valid() || record.ExpiresAt.IsZero() {
		return "", ErrNonceInvalid
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", ErrNonceInvalid
	}
	nonce := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]NonceRecord)
	}
	if s.used == nil {
		s.used = make(map[string]time.Time)
	}
	for len(s.entries)+len(s.used) >= MaxMemoryNonces {
		var oldest string
		var oldestExpiry time.Time
		for value, item := range s.entries {
			if oldest == "" || item.ExpiresAt.Before(oldestExpiry) {
				oldest, oldestExpiry = value, item.ExpiresAt
			}
		}
		for value, expiry := range s.used {
			if oldest == "" || expiry.Before(oldestExpiry) {
				oldest, oldestExpiry = value, expiry
			}
		}
		if oldest == "" {
			return "", ErrNonceInvalid
		}
		delete(s.entries, oldest)
		delete(s.used, oldest)
	}
	s.entries[nonce] = record
	return nonce, nil
}

func (s *MemoryNonceStore) Consume(_ context.Context, nonce string, now time.Time) (NonceRecord, error) {
	if s == nil || nonce == "" || now.IsZero() {
		return NonceRecord{}, ErrNonceInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for value, record := range s.entries {
		if !now.Before(record.ExpiresAt) {
			delete(s.entries, value)
		}
	}
	for value, expiry := range s.used {
		if !now.Before(expiry) {
			delete(s.used, value)
		}
	}
	if _, ok := s.used[nonce]; ok {
		return NonceRecord{}, ErrNonceReplay
	}
	record, ok := s.entries[nonce]
	if !ok {
		return NonceRecord{}, ErrNonceInvalid
	}
	delete(s.entries, nonce)
	s.used[nonce] = record.ExpiresAt
	if !now.Before(record.ExpiresAt) {
		return NonceRecord{}, ErrNonceInvalid
	}
	return record, nil
}
