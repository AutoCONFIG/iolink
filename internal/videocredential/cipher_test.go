package videocredential_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
)

func binding(t *testing.T, purpose domain.VideoCredentialPurpose, tenant, entity, version int64) domain.VideoCredentialBinding {
	t.Helper()
	b, err := domain.NewVideoCredentialBinding(purpose, tenant, entity, version)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newCipher(t *testing.T, root byte) *videocredential.Cipher {
	t.Helper()
	c, err := videocredential.New(bytes.Repeat([]byte{root}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestVideoCredentialRoundTripMaximumUnicodeFields(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	plaintext, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{strings.Repeat("\U0001f642", 128), strings.Repeat("\U0001f642", 128)})
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := c.Seal(t.Context(), b, plaintext)
	if err != nil {
		t.Fatalf("maximum contract fields rejected: %v", err)
	}
	got, err := c.Open(t.Context(), b, sealed)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("maximum contract fields round trip failed: %v", err)
	}
}

func TestVideoCredentialRoundTrip(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	plaintext := []byte(`{"username":"operator","password":"test-camera-secret"}`)
	sealed, err := c.Seal(t.Context(), b, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, plaintext) || len(sealed) != len(plaintext)+28 {
		t.Fatal("credential encryption lacks ciphertext envelope")
	}
	got, err := newCipher(t, 0x51).Open(t.Context(), b, sealed)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("credential round trip failed: %v", err)
	}
}

func TestVideoCredentialSealUsesDistinctRandomNonces(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	seen := make(map[string]struct{})
	for range 32 {
		sealed, err := c.Seal(t.Context(), b, []byte("test-camera-secret"))
		if err != nil {
			t.Fatal(err)
		}

		nonce := string(sealed[:12])
		if _, exists := seen[nonce]; exists {
			t.Fatal("credential nonce repeated")
		}
		seen[nonce] = struct{}{}
	}
}

func TestVideoCredentialRejectsWrongKeyAndBinding(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	sealed, err := c.Seal(t.Context(), b, []byte("test-camera-secret"))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		cipher  *videocredential.Cipher
		binding domain.VideoCredentialBinding
	}{
		{newCipher(t, 0x52), b},
		{c, binding(t, domain.CameraCredential, 2, 2, 3)},
		{c, binding(t, domain.CameraCredential, 1, 3, 3)},
		{c, binding(t, domain.CameraCredential, 1, 2, 4)},
		{c, binding(t, domain.GBDeviceCredential, 1, 2, 3)},
	} {
		got, err := item.cipher.Open(t.Context(), item.binding, sealed)
		if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
			t.Fatalf("wrong key or binding accepted: %v", err)
		}
	}
}

func TestVideoCredentialRejectsTamperedAndTruncatedEnvelope(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	sealed, err := c.Seal(t.Context(), b, []byte("test-camera-secret"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range sealed {
		changed := bytes.Clone(sealed)
		changed[i] ^= 1
		got, err := c.Open(t.Context(), b, changed)
		if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
			t.Fatal("tampered credential accepted")
		}
	}
	for i := 0; i < len(sealed); i++ {
		got, err := c.Open(t.Context(), b, sealed[:i])
		if !errors.Is(err, domain.ErrVideoCredentialUnavailable) || got != nil {
			t.Fatal("truncated credential accepted")
		}
	}
}

func TestVideoCredentialFailsClosedOnInvalidInputs(t *testing.T) {
	if _, err := videocredential.New(make([]byte, 31)); !errors.Is(err, domain.ErrVideoCredentialUnavailable) {
		t.Fatal("short root accepted")
	}
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	for _, raw := range [][]byte{nil, {}, make([]byte, 4097)} {
		if got, err := c.Seal(t.Context(), b, raw); !errors.Is(err, domain.ErrInvalidVideoCredential) || got != nil {
			t.Fatal("invalid plaintext accepted")
		}
	}
	if _, err := c.Seal(t.Context(), domain.VideoCredentialBinding{}, []byte("x")); !errors.Is(err, domain.ErrInvalidVideoCredential) {
		t.Fatal("zero binding accepted")
	}
	var unavailable *videocredential.Cipher
	if _, err := unavailable.Open(t.Context(), b, make([]byte, 29)); !errors.Is(err, domain.ErrVideoCredentialUnavailable) {
		t.Fatal("nil cipher accepted")
	}
	for _, purpose := range []domain.VideoCredentialPurpose{"", "other"} {
		if _, err := domain.NewVideoCredentialBinding(purpose, 1, 2, 3); !errors.Is(err, domain.ErrInvalidVideoCredential) {
			t.Fatal("invalid purpose accepted")
		}
	}
	for _, ids := range [][3]int64{{0, 2, 3}, {1, 0, 3}, {1, 2, 0}, {-1, 2, 3}} {
		if _, err := domain.NewVideoCredentialBinding(domain.CameraCredential, ids[0], ids[1], ids[2]); !errors.Is(err, domain.ErrInvalidVideoCredential) {
			t.Fatal("invalid binding IDs accepted")
		}
	}
}

func TestVideoCredentialHonorsCanceledContext(t *testing.T) {
	c := newCipher(t, 0x51)
	b := binding(t, domain.CameraCredential, 1, 2, 3)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.Seal(ctx, b, []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled encryption accepted")
	}
	if _, err := c.Open(ctx, b, make([]byte, 29)); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled decryption accepted")
	}
}
