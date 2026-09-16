package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// adminPasswordSalt 与文档约定的管理员口令哈希盐;修改会使所有已存口令失效。
const adminPasswordSalt = "iolink-admin"

// HashPassword returns sha256 hex of salt:password (admin accounts).
func HashPassword(password string) string {
	sum := sha256.Sum256([]byte(adminPasswordSalt + ":" + password))
	return hex.EncodeToString(sum[:])
}

// CheckPassword verifies a stored hash against a plaintext password.
func CheckPassword(storedHash, password string) bool {
	return storedHash == HashPassword(password)
}

// DeriveAdminKey derives the admin JWT signing key from the main secret,
// so app (/api/v1) and admin (/admin/v1) tokens can never be interchanged.
func DeriveAdminKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("admin-v1"))
	return mac.Sum(nil)
}
