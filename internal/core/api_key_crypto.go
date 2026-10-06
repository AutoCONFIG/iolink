package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

func (s *Service) rootSecret() []byte { return append([]byte(nil), s.apiKeyRoot...) }

func randomKeyID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("random key id: %w", err)
	}
	return "ik_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func encryptAPISecret(root, secret []byte) ([]byte, []byte, error) {
	if len(root) < 32 {
		return nil, nil, errors.New("api key encryption key unavailable")
	}
	key := sha256.Sum256(append(append([]byte{}, root...), []byte(":api-key")...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, fmt.Errorf("api key cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("api key gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("api key nonce: %w", err)
	}
	return gcm.Seal(nil, nonce, secret, nil), nonce, nil
}

func decryptAPISecret(root, ciphertext, nonce []byte) ([]byte, error) {
	if len(root) < 32 {
		return nil, errors.New("api key encryption key unavailable")
	}
	key := sha256.Sum256(append(append([]byte{}, root...), []byte(":api-key")...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("api key cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("api key gcm: %w", err)
	}
	secret, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("api key secret unavailable")
	}
	return secret, nil
}
