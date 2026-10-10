package media

import (
	"context"
	"errors"
	"time"
)

const (
	MaxManifestBytes = 256 * 1024
	MaxSegmentBytes  = 8 * 1024 * 1024
)

var (
	ErrInvalid            = errors.New("invalid media request")
	ErrUnavailable        = errors.New("media unavailable")
	ErrUnauthorized       = errors.New("media unauthorized")
	ErrForbidden          = errors.New("media forbidden")
	ErrNotFound           = errors.New("media not found")
	ErrConflict           = errors.New("media conflict")
	ErrPending            = errors.New("media pending")
	ErrDisabled           = errors.New("media provider disabled")
	ErrUnsupported        = errors.New("media unsupported")
	ErrRetryable          = errors.New("media retryable failure")
	ErrUnknown            = errors.New("media result unknown")
	ErrLeaseLost          = errors.New("media lease lost")
	ErrPersistenceUnknown = errors.New("media persistence result unknown")
	ErrLicenseRequired    = errors.New("video license required")
	ErrLicenseUnavailable = errors.New("video license unavailable")
	ErrBadMedia           = errors.New("bad media")
)

type JobKind string

const (
	JobStart JobKind = "start"
	JobStop  JobKind = "stop"
)

type JobStatus string

const (
	JobReady     JobStatus = "ready"
	JobStopped   JobStatus = "stopped"
	JobSkipped   JobStatus = "skipped"
	JobRetryable JobStatus = "retryable"
	JobFailed    JobStatus = "failed"
	JobUnknown   JobStatus = "unknown"
	JobDisabled  JobStatus = "disabled"
)

type LicenseSnapshot struct {
	DeploymentID string
	Payload      []byte
	Signature    []byte
	SHA256       string
	MaxSeenAt    time.Time
	ClockError   bool
	Now          time.Time
}

type PlaybackTarget struct {
	TenantID                int64
	CameraID                int64
	SourceVersion           int64
	StreamID                string
	UserID                  int64
	UserTokenVersion        int64
	TenantPermissionVersion int64
	MemberPermissionVersion int64
	License                 LicenseSnapshot
}

type SessionState string

const (
	SessionPending SessionState = "pending"
	SessionReady   SessionState = "ready"
	SessionFailed  SessionState = "failed"
	SessionRevoked SessionState = "revoked"
	SessionExpired SessionState = "expired"
)

type Session struct {
	ID                      string
	StreamID                string
	TenantID                int64
	CameraID                int64
	SourceVersion           int64
	UserID                  int64
	UserTokenVersion        int64
	TenantPermissionVersion int64
	MemberPermissionVersion int64
	State                   SessionState
	ErrorCode               string
	CreatedAt               time.Time
	ExpiresAt               time.Time
}

type CreateSession struct {
	Session     Session
	Target      PlaybackTarget
	TokenHash   [32]byte
	StartJobKey string
}

type Job struct {
	ID            string
	Kind          JobKind
	CameraID      int64
	SourceVersion int64
	StreamID      string
	SessionID     string
	Attempts      int
}

type Lease struct {
	Job
	LeaseToken  string
	LeasedUntil time.Time
}

type JobAuthorization struct {
	Target         PlaybackTarget
	Active         bool
	SourceCurrent  bool
	StopSuperseded bool
}

type JobResult struct {
	Status    JobStatus
	ErrorCode string
	NextAt    time.Time
}

type MediaClaims struct {
	SessionID     string
	TenantID      int64
	CameraID      int64
	UserID        int64
	SourceVersion int64
	Audience      string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	JTI           string
}

type IssuedToken struct {
	Raw    string
	Claims MediaClaims
}

type MediaAccess struct {
	SessionID     string
	StreamID      string
	TenantID      int64
	CameraID      int64
	SourceVersion int64
	State         SessionState
	ExpiresAt     time.Time
}

type Store interface {
	PreparePlayback(context.Context, int64) (PlaybackTarget, error)
	CreatePlayback(context.Context, CreateSession) (Session, error)
	ReadOwnedPlayback(context.Context, string) (Session, error)
	StopOwnedPlayback(context.Context, string, time.Time) error

	ClaimJob(context.Context, time.Time, time.Duration, string) (Lease, bool, error)
	RevalidateJob(context.Context, Lease) (JobAuthorization, error)
	FinishJob(context.Context, Lease, JobResult, time.Time) error
	AuthorizeMedia(context.Context, MediaClaims, [32]byte, time.Time) (MediaAccess, error)
}

type LicenseGuard interface {
	RequireVideo(context.Context, LicenseSnapshot) error
}

type TokenIssuer interface {
	Issue(context.Context, MediaClaims) (IssuedToken, error)
	Verify(context.Context, string) (MediaClaims, error)
}

type Provider interface {
	Start(context.Context, StartRequest) error
	Stop(context.Context, StopRequest) error
}

type ManifestSource interface {
	FetchManifest(context.Context, MediaAccess) ([]byte, error)
}

type SegmentSource interface {
	FetchSegment(context.Context, MediaAccess, string) ([]byte, error)
}

type SegmentMapping struct {
	ID            string
	SessionID     string
	StreamID      string
	SourceVersion int64
	Path          string
}

type SegmentMappingStore interface {
	CreateSegmentMappings(context.Context, []SegmentMapping) ([]SegmentMapping, error)
	ResolveSegmentMapping(context.Context, string, string) (SegmentMapping, error)
}

type StartRequest struct {
	CameraID      int64
	SourceVersion int64
	StreamID      string
}

type StopRequest struct {
	CameraID      int64
	SourceVersion int64
	StreamID      string
}
