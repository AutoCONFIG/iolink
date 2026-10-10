package videocredential_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"testing"
)

func TestVideoCredentialEnvelopeUsesAES256GCMWithIndependentHKDFKey(t *testing.T) {
	root := bytes.Repeat([]byte{0x51}, 32)
	plaintext := []byte("test-camera-secret")
	c := newCipher(t, 0x51)
	b := binding(t, "camera", 1, 2, 3)
	sealed, err := c.Seal(t.Context(), b, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	key := independentCameraKey(root)
	defer clear(key)
	aad := append([]byte("iolink-camera-credential-v1:camera:"), []byte{
		0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 2,
		0, 0, 0, 0, 0, 0, 0, 3,
	}...)

	for _, item := range []struct {
		name string
		key  []byte
		want bool
	}{
		{"derived camera key", key, true},
		{"root key directly", root, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			block, err := aes.NewCipher(item.key)
			if err != nil {
				t.Fatal(err)
			}
			gcm, err := cipher.NewGCM(block)
			if err != nil {
				t.Fatal(err)
			}

			got, err := gcm.Open(nil, sealed[:12], sealed[12:], aad)
			if item.want {
				if err != nil || !bytes.Equal(got, plaintext) {
					t.Fatalf("independent AES-256-GCM decoder failed: %v", err)
				}
				return
			}
			if err == nil || got != nil {
				t.Fatal("credential encrypted with root key directly")
			}
		})
	}
}

func TestVideoCredentialOpensIndependentAES256GCMEnvelope(t *testing.T) {
	// Given: an envelope produced without the adapter's HKDF or random-nonce API.
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
	aad := append([]byte("iolink-camera-credential-v1:camera:"), []byte{
		0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 2,
		0, 0, 0, 0, 0, 0, 0, 3,
	}...)
	plaintext := []byte("independently-encrypted-camera-secret")
	envelope := gcm.Seal(bytes.Clone(nonce), nonce, plaintext, aad)
	c := newCipher(t, 0x51)
	b := binding(t, "camera", 1, 2, 3)

	// When
	got, err := c.Open(t.Context(), b, envelope)

	// Then
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("independent envelope rejected: %v", err)
	}
}

func independentCameraKey(root []byte) []byte {
	// RFC 5869 Extract and the first Expand block, independently of crypto/hkdf.
	extract := hmac.New(sha256.New, make([]byte, sha256.Size))
	extract.Write(root)
	prk := extract.Sum(nil)
	defer clear(prk)
	expand := hmac.New(sha256.New, prk)
	expand.Write(append([]byte("iolink-camera-credential-v1"), 1))
	return expand.Sum(nil)
}
