package domain

import (
	"context"
	"encoding/binary"
	"errors"
)

var (
	ErrInvalidVideoCredential     = errors.New("invalid video credential")
	ErrVideoCredentialUnavailable = errors.New("video credential unavailable")
	ErrVideoSecretSerialization   = errors.New("video secret cannot be serialized")
)

type VideoCredentialPurpose string

const (
	CameraCredential   VideoCredentialPurpose = "camera"
	GBDeviceCredential VideoCredentialPurpose = "gb_device"
)

// VideoCredentialBinding prevents moving ciphertext between tenants, entities,
// credential versions or the camera and GB device namespaces.
type VideoCredentialBinding struct {
	purpose                     VideoCredentialPurpose
	tenantID, entityID, version int64
}

func NewVideoCredentialBinding(purpose VideoCredentialPurpose, tenantID, entityID, version int64) (VideoCredentialBinding, error) {
	if (purpose != CameraCredential && purpose != GBDeviceCredential) || tenantID <= 0 || entityID <= 0 || version <= 0 {
		return VideoCredentialBinding{}, ErrInvalidVideoCredential
	}
	return VideoCredentialBinding{purpose, tenantID, entityID, version}, nil
}

func (b VideoCredentialBinding) Valid() bool {
	return (b.purpose == CameraCredential || b.purpose == GBDeviceCredential) && b.tenantID > 0 && b.entityID > 0 && b.version > 0
}

func (b VideoCredentialBinding) AssociatedData() []byte {
	out := []byte("iolink-camera-credential-v1:" + string(b.purpose) + ":")
	for _, value := range []int64{b.tenantID, b.entityID, b.version} {
		out = binary.BigEndian.AppendUint64(out, uint64(value))
	}
	return out
}

type VideoCredentialCipher interface {
	Seal(ctx context.Context, binding VideoCredentialBinding, plaintext []byte) ([]byte, error)
	Open(ctx context.Context, binding VideoCredentialBinding, ciphertext []byte) ([]byte, error)
}
