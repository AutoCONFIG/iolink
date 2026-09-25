package platform

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// HashPassword returns a salted Argon2id PHC string. The legacy SHA256 verifier
// is retained only to support controlled upgrades of existing accounts.
func HashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic("secure random unavailable")
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}
func IsLegacyPassword(stored string) bool {
	if len(stored) != 64 {
		return false
	}
	_, err := hex.DecodeString(stored)
	return err == nil
}
func CheckPassword(stored, password string) bool {
	if IsLegacyPassword(stored) {
		sum := sha256.Sum256([]byte("iolink-admin:" + password))
		return subtle.ConstantTimeCompare([]byte(stored), []byte(hex.EncodeToString(sum[:]))) == 1
	}
	parts := strings.Split(stored, "$")
	// Accept only our bounded parameters, so a corrupt hash cannot allocate arbitrary memory.
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
func DeriveAdminKey(secret string) []byte { return deriveKey(secret, "admin-v1") }
func DeriveAppKey(secret string) []byte   { return deriveKey(secret, "app-v1") }
func deriveKey(secret, label string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(label))
	return m.Sum(nil)
}
