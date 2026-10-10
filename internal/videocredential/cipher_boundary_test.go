package videocredential_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
)

func TestVideoCredentialRoundTripAtSizeBoundaries(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	for _, size := range []int{1, 4096} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			plaintext := bytes.Repeat([]byte{0xa5}, size)
			sealed, err := c.Seal(t.Context(), b, plaintext)
			if err != nil {
				t.Fatal(err)
			}

			got, err := c.Open(t.Context(), b, sealed)
			if err != nil || !bytes.Equal(got, plaintext) {
				t.Fatalf("credential boundary round trip failed: %v", err)
			}
		})
	}
}

func TestVideoCredentialRoundTripEscapedMaximumUnicodeFields(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	field := strings.Repeat(`\ud83d\ude42`, 128)
	plaintext := []byte(`{"username":"` + field + `","password":"` + field + `"}`)
	sealed, err := c.Seal(t.Context(), b, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Open(t.Context(), b, sealed)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("escaped maximum contract fields rejected: %v", err)
	}
}

func TestVideoCredentialRejectsZeroInstancesAndBinding(t *testing.T) {
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	for _, c := range []*videocredential.Cipher{nil, {}} {
		got, err := c.Seal(t.Context(), b, []byte("x"))
		if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
			t.Fatal("unavailable cipher encrypted")
		}
		got, err = c.Open(t.Context(), b, make([]byte, 29))
		if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
			t.Fatal("unavailable cipher decrypted")
		}
	}
	c := newCipher(t, 0x51)
	sealed, err := c.Seal(t.Context(), b, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Open(t.Context(), domain.VideoCredentialBinding{}, sealed)
	if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
		t.Fatal("zero binding decrypted")
	}
}

func TestVideoCredentialRejectsOversizedEnvelope(t *testing.T) {
	// Given: the independently authenticated envelope has valid key, nonce and AAD,
	// but its 4097-byte plaintext exceeds the credential limit by one byte.
	key := independentCameraKey(bytes.Repeat([]byte{0x51}, 32))
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{0x27}, gcm.NonceSize())
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	plaintext := bytes.Repeat([]byte{0xa5}, 4097)
	envelope := gcm.Seal(bytes.Clone(nonce), nonce, plaintext, b.AssociatedData())
	if len(envelope) != 4125 {
		t.Fatal("unexpected envelope size")
	}
	decoded, err := gcm.Open(nil, envelope[:12], envelope[12:], b.AssociatedData())
	if err != nil || !bytes.Equal(decoded, plaintext) {
		t.Fatal("independent envelope is unauthenticated")
	}
	c := newCipher(t, 0x51)

	// When
	got, err := c.Open(t.Context(), b, envelope)

	// Then
	if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
		t.Fatal("oversized authenticated ciphertext accepted")
	}
}

func TestVideoCredentialHonorsExpiredDeadline(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer cancel()

	got, err := c.Seal(ctx, b, []byte("x"))
	if !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("expired encryption accepted")
	}
	got, err = c.Open(ctx, b, make([]byte, 29))
	if !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("expired decryption accepted")
	}
}
