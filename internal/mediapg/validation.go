package mediapg

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func validCreate(in media.CreateSession) bool {
	return validUUID(in.Session.ID) && validUUID(in.Session.StreamID) && in.Session.TenantID > 0 && in.Session.CameraID > 0 && in.Session.UserID > 0 && in.Session.SourceVersion > 0 && !in.Session.CreatedAt.IsZero() && !in.Session.ExpiresAt.IsZero() && in.Session.ExpiresAt.After(in.Session.CreatedAt) && in.Session.ExpiresAt.Sub(in.Session.CreatedAt) <= 5*time.Minute && !zeroHash(in.TokenHash) && in.StartJobKey != "" && len(in.StartJobKey) <= maxJobKey
}

func validLease(l media.Lease) bool {
	if l.ID == "" || !validUUID(l.SessionID) || !validUUID(l.StreamID) || l.CameraID <= 0 || l.SourceVersion <= 0 || l.LeaseToken == "" || l.LeasedUntil.IsZero() {
		return false
	}
	return l.Kind == media.JobStart || l.Kind == media.JobStop
}

func validWorkerID(workerID string) bool {
	if workerID == "" || len(workerID) > maxWorkerID || strings.TrimSpace(workerID) != workerID {
		return false
	}
	for i := range workerID {
		if workerID[i] < 0x20 || workerID[i] == 0x7f {
			return false
		}
	}
	return true
}

func validJobResult(r media.JobResult) bool {
	switch r.Status {
	case media.JobReady, media.JobStopped, media.JobSkipped, media.JobRetryable, media.JobFailed, media.JobUnknown, media.JobDisabled:
		return true
	default:
		return false
	}
}

func validClaims(c media.MediaClaims) bool {
	return validUUID(c.SessionID) && c.TenantID > 0 && c.CameraID > 0 && c.UserID > 0 && c.SourceVersion > 0 && c.Audience == "iolink-media-v1" && c.JTI != "" && !c.IssuedAt.IsZero() && !c.ExpiresAt.IsZero() && c.ExpiresAt.After(c.IssuedAt)
}

func validUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func validMapping(m media.SegmentMapping) bool {
	return validUUID(m.ID) && validUUID(m.SessionID) && validUUID(m.StreamID) && m.SourceVersion > 0 && m.Path != "" && !strings.ContainsAny(m.Path, `%?#\\`) && !strings.HasPrefix(m.Path, "/") && !strings.Contains(m.Path, "..")
}

func zeroHash(b [32]byte) bool {
	var z [32]byte
	return b == z
}

func equalHash(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func newLeaseToken() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, media.ErrInvalid) || errors.Is(err, media.ErrForbidden) || errors.Is(err, media.ErrNotFound) || errors.Is(err, media.ErrConflict) || errors.Is(err, media.ErrLicenseUnavailable) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return media.ErrConflict
		case "23514":
			return media.ErrInvalid
		case "23503":
			return media.ErrNotFound
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	}
	return media.ErrUnavailable
}

func jobStatus(status media.JobStatus) string {
	switch status {
	case media.JobReady, media.JobStopped:
		return "succeeded"
	case media.JobRetryable:
		return "retryable"
	case media.JobSkipped, media.JobDisabled:
		return "cancelled"
	default:
		return "failed"
	}
}

func sessionErrorCode(code string) string {
	switch code {
	case "source_unreachable", "unsupported_codec", "provider_unavailable", "registration_expired", "catalog_stale":
		return code
	default:
		return "provider_unavailable"
	}
}
