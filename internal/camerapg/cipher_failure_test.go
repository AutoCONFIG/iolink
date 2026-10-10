package camerapg_test

import (
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
)

func TestCreate_whenCredentialCipherFails(t *testing.T) {
	for _, cipher := range []domain.VideoCredentialCipher{(*videocredential.Cipher)(nil), &videocredential.Cipher{}} {
		// Given
		f := newFixture(t)
		f.deps.Cipher = cipher
		s := camera.New(f.deps)
		// When
		_, err := s.Create(actor(1, "owner"), configuration(t, 101, true))
		// Then
		if !errors.Is(err, camera.ErrUnavailable) {
			t.Fatalf("error=%v", err)
		}
		var cameras, audits int
		if err = f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM video_cameras),(SELECT count(*) FROM audit_events WHERE resource_type='camera')`).Scan(&cameras, &audits); err != nil || cameras != 3 || audits != 0 {
			t.Fatalf("cameras=%d audits=%d error=%v", cameras, audits, err)
		}
	}
}
