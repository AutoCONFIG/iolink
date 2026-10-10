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
	used    map[string]struct{}
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{entries: make(map[string]NonceRecord), used: make(map[string]struct{})}
}

func (s *MemoryNonceStore) Issue(_ context.Context, record NonceRecord) (string, error) {
	if s == nil || record.ExpiresAt.IsZero() {
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
		s.used = make(map[string]struct{})
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
	if _, ok := s.used[nonce]; ok {
		return NonceRecord{}, ErrNonceReplay
	}
	record, ok := s.entries[nonce]
	if !ok {
		return NonceRecord{}, ErrNonceInvalid
	}
	delete(s.entries, nonce)
	s.used[nonce] = struct{}{}
	if !now.Before(record.ExpiresAt) {
		return NonceRecord{}, ErrNonceInvalid
	}
	return record, nil
}
