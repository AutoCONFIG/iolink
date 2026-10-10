package mediajobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
)

const (
	MaxAttempts = 6
	maxLease    = 5 * time.Minute
	maxWorkerID = 128
)

var (
	ErrInvalidJob = errors.New("invalid media job")
	ErrNotFound   = errors.New("media job not found")
)

func contextError(ctx context.Context) error {
	if ctx == nil {
		return context.Canceled
	}
	return ctx.Err()
}

func validLease(lease media.Lease) bool {
	return lease.ID != "" && lease.LeaseToken != "" && lease.SourceVersion > 0 && lease.CameraID > 0 && lease.StreamID != ""
}

func leaseMatches(j *entry, lease media.Lease) bool {
	return j != nil && j.leaseToken == lease.LeaseToken && j.leaseOwner != "" && j.job.ID == lease.ID && j.job.Kind == lease.Kind && j.job.CameraID == lease.CameraID && j.job.SourceVersion == lease.SourceVersion && j.job.StreamID == lease.StreamID && j.job.SessionID == lease.SessionID
}

func validResult(result media.JobResult) bool {
	switch result.Status {
	case media.JobReady, media.JobStopped, media.JobSkipped, media.JobRetryable, media.JobFailed, media.JobUnknown, media.JobDisabled:
		return result.Status != media.JobRetryable || !result.NextAt.IsZero()
	default:
		return false
	}
}

func resultState(status media.JobStatus) string {
	switch status {
	case media.JobRetryable:
		return "retryable"
	case media.JobFailed, media.JobDisabled:
		return "failed"
	case media.JobUnknown:
		return "unknown"
	case media.JobSkipped:
		return "cancelled"
	default:
		return "succeeded"
	}
}

func leaseToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
