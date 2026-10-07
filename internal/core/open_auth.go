package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrOpenUnauthorized = errors.New("open api authentication failed")
	ErrOpenReplay       = errors.New("open api nonce replay")
)

type OpenRateLimitError struct{ RetryAfter int }

func (e *OpenRateLimitError) Error() string        { return "open api rate limit exceeded" }
func (e *OpenRateLimitError) RetryAfterValue() int { return e.RetryAfter }

func CanonicalOpenQuery(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	type pair struct{ key, value string }
	pairs := make([]pair, 0)
	for _, item := range strings.Split(raw, "&") {
		parts := strings.SplitN(item, "=", 2)
		key, err := url.PathUnescape(parts[0])
		if err != nil {
			return "", errors.New("invalid query encoding")
		}
		value := ""
		if len(parts) == 2 {
			value, err = url.PathUnescape(parts[1])
			if err != nil {
				return "", errors.New("invalid query encoding")
			}
		}
		pairs = append(pairs, pair{key: rfc3986(key), value: rfc3986(value)})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].key == pairs[j].key {
			return pairs[i].value < pairs[j].value
		}
		return pairs[i].key < pairs[j].key
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.key + "=" + p.value
	}
	return strings.Join(parts, "&"), nil
}

func CanonicalOpenPath(path string) (string, error) {
	if path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "\\") {
		return "", errors.New("invalid path")
	}
	if strings.Contains(strings.ToLower(path), "%2e") || strings.Contains(strings.ToLower(path), "%2f") || strings.Contains(strings.ToLower(path), "%5c") {
		return "", errors.New("ambiguous path")
	}
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return "", errors.New("invalid path encoding")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("ambiguous path")
		}
	}
	return path, nil
}

func (s *Service) AuthenticateOpen(ctx context.Context, req domain.OpenRequest, now time.Time) (domain.OpenPrincipal, error) {
	if err := s.RequireLicenseFeature(ctx, "openapi"); err != nil {
		return domain.OpenPrincipal{}, err
	}
	if len(s.apiKeyRoot) < 32 || req.KeyID == "" || req.Timestamp == 0 || len(req.Nonce) < 16 || req.Signature == "" {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	path, err := CanonicalOpenPath(req.Path)
	if err != nil {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	query, err := CanonicalOpenQuery(req.Query)
	if err != nil {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	delta := uint64(req.Timestamp) - uint64(now.Unix())
	if req.Timestamp < now.Unix() {
		delta = uint64(now.Unix()) - uint64(req.Timestamp)
	}
	if delta > 300 {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	sig, err := hex.DecodeString(req.Signature)
	if err != nil || len(sig) != sha256.Size || strings.ToLower(req.Signature) != req.Signature {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.OpenPrincipal{}, fmt.Errorf("begin open authentication: %w", err)
	}
	defer tx.Rollback(ctx)
	var tenantID int64
	var scopes []string
	var resourcesRaw []byte
	var encrypted, nonce []byte
	if err := tx.QueryRow(ctx, `SELECT tenant_id,scopes,resources,encrypted_secret,secret_nonce FROM api_keys WHERE key_id=$1 AND revoked_at IS NULL FOR UPDATE`, req.KeyID).Scan(&tenantID, &scopes, &resourcesRaw, &encrypted, &nonce); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OpenPrincipal{}, ErrOpenUnauthorized
		}
		return domain.OpenPrincipal{}, fmt.Errorf("load open api key: %w", err)
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT active FROM tenants WHERE id=$1`, tenantID).Scan(&active); err != nil || !active {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	secret, err := decryptAPISecret(s.apiKeyRoot, encrypted, nonce)
	if err != nil {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	bodyHash := sha256.Sum256(req.Body)
	signed := strings.Join([]string{strings.ToUpper(req.Method), path, query, fmt.Sprint(req.Timestamp), req.Nonce, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signed))
	if subtle.ConstantTimeCompare(mac.Sum(nil), sig) != 1 {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	var resources domain.APIKeyResourceScope
	if err := json.Unmarshal(resourcesRaw, &resources); err != nil {
		return domain.OpenPrincipal{}, ErrOpenUnauthorized
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_key_nonces WHERE expires_at < now()`); err != nil {
		return domain.OpenPrincipal{}, fmt.Errorf("expire open nonces: %w", err)
	}
	nonceHash := sha256.Sum256([]byte(req.Nonce))
	if _, err := tx.Exec(ctx, `INSERT INTO api_key_nonces(key_id,nonce_hash,expires_at) VALUES($1,$2,now()+interval '10 minutes')`, req.KeyID, nonceHash[:]); err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return domain.OpenPrincipal{}, ErrOpenReplay
		}
		return domain.OpenPrincipal{}, fmt.Errorf("store open nonce: %w", err)
	}
	var tokens float64
	var last time.Time
	if err := tx.QueryRow(ctx, `SELECT rate_tokens,rate_last_refill FROM api_keys WHERE key_id=$1 FOR UPDATE`, req.KeyID).Scan(&tokens, &last); err != nil {
		return domain.OpenPrincipal{}, fmt.Errorf("load open rate: %w", err)
	}
	remaining, retryAfter := consumeOpenRate(tokens, last, now, s.openRatePerMinute, s.openRateBurst)
	if retryAfter > 0 {
		return domain.OpenPrincipal{}, &OpenRateLimitError{RetryAfter: retryAfter}
	}
	tokens = remaining
	if _, err := tx.Exec(ctx, `UPDATE api_keys SET rate_tokens=$2,rate_last_refill=$3 WHERE key_id=$1`, req.KeyID, tokens, now); err != nil {
		return domain.OpenPrincipal{}, fmt.Errorf("update open rate: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.OpenPrincipal{}, fmt.Errorf("commit open authentication: %w", err)
	}
	return domain.OpenPrincipal{KeyID: req.KeyID, TenantID: tenantID, Scopes: scopes, Resources: resources}, nil
}

func rfc3986(value string) string {
	const hexChars = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("-._~", rune(c)) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexChars[c>>4])
			b.WriteByte(hexChars[c&15])
		}
	}
	return b.String()
}
