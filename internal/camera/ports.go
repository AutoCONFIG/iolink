package camera

import (
	"context"
	"errors"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

var (
	ErrUnavailable = errors.New("camera unavailable")
	ErrInternal    = errors.New("camera operation failed")
)

type Camera struct {
	ID            int64                  `json:"id"`
	PondID        int64                  `json:"pond_id"`
	Name          string                 `json:"name"`
	SourceKind    domain.VideoSourceKind `json:"source_kind"`
	SourceVersion int64                  `json:"source_version"`
	GBDeviceID    *int64                 `json:"gb_device_id,omitempty"`
	GBChannelID   *string                `json:"gb_channel_id,omitempty"`
	Status        string                 `json:"status"`
	ErrorCode     *string                `json:"error_code,omitempty"`
}
type List struct {
	Items       []Camera `json:"items"`
	NextAfterID *int64   `json:"next_after_id,omitempty"`
}
type Page struct {
	Limit   int
	AfterID int64
}
type (
	Target  struct{ CameraID, PondID int64 }
	Binding struct {
		ID, FarmID, SourceVersion int64
		Enabled                   bool
	}
)

type Mutation struct {
	Binding          Binding
	Configuration    Configuration
	CredentialCipher []byte
}

func (Mutation) String() string               { return "camera mutation [redacted]" }
func (Mutation) GoString() string             { return "camera mutation [redacted]" }
func (Mutation) MarshalJSON() ([]byte, error) { return nil, domain.ErrVideoSecretSerialization }

// Store owns transaction boundaries. Every operation checks authority in that
// transaction; Prepare locks farms before the camera and derives scope from pond.
type CameraStore interface {
	Run(context.Context, func(CameraTransaction) error) error
}
type CameraTransaction interface {
	Authorize(context.Context, bool) error
	LicenseSnapshot(context.Context) (LicenseSnapshot, error)
	List(context.Context, Page) (List, error)
	Read(context.Context, int64) (Camera, error)
	CheckTarget(context.Context, Target) error
	Prepare(context.Context, Target) (Binding, error)
	Write(context.Context, Mutation) (Camera, error)
	Disable(context.Context, Binding) error
}
type API interface {
	List(context.Context, Page) (List, error)
	Get(context.Context, int64) (Camera, error)
	Create(context.Context, Configuration) (Camera, error)
	Replace(context.Context, int64, Configuration) (Camera, error)
	Disable(context.Context, int64) error
}

// Availability is injected only when the complete runtime can manage the source.
type Availability interface {
	RequireConfiguration(context.Context, domain.VideoSourceKind) error
}

// LicenseClock preserves the existing durable high-water mark in its own short
// transaction, before the mutation transaction borrows a connection.
type LicenseClock interface{ ObserveLicenseClock(context.Context) error }
