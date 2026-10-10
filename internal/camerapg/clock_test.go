package camerapg_test

import (
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestConfigure_whenUnauthorized_preservesClock(t *testing.T) {
	// Given
	f := newFixture(t)
	// When
	_, err := f.service.Create(actor(3, "member"), configuration(t, 101, false))
	// Then
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
	var untouched bool
	if err = f.pool.QueryRow(t.Context(), `SELECT max_seen_at IS NULL FROM license_clock`).Scan(&untouched); err != nil || !untouched {
		t.Fatal("unauthorized configuration invoked clock observer")
	}
}

func TestConfigure_whenClockRollback_latchesDespiteRejection(t *testing.T) {
	// Given
	f := newFixture(t)
	exec(t, f.pool, `UPDATE license_clock SET max_seen_at=now()+interval '10 minutes'`)
	// When
	_, err := f.service.Create(actor(1, "owner"), configuration(t, 101, false))
	// Then
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
	var latched bool
	var cameras, audits int
	if err = f.pool.QueryRow(t.Context(), `SELECT clock_error,(SELECT count(*) FROM video_cameras),(SELECT count(*) FROM audit_events WHERE resource_type='camera') FROM license_clock`).Scan(&latched, &cameras, &audits); err != nil || !latched || cameras != 3 || audits != 0 {
		t.Fatalf("latched=%v cameras=%d audits=%d error=%v", latched, cameras, audits, err)
	}
}

func TestConfigure_whenBusinessMutationRollsBack_preservesClockObservation(t *testing.T) {
	// Given
	f := newFixture(t)
	exec(t, f.pool, `CREATE FUNCTION reject_camera_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit failed'; END $$; CREATE TRIGGER reject_camera BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_camera_audit()`)
	// When
	_, err := f.service.Create(actor(1, "owner"), configuration(t, 101, false))
	// Then
	if !errors.Is(err, camera.ErrInternal) {
		t.Fatalf("error=%v", err)
	}
	var observed bool
	var cameras int
	if err = f.pool.QueryRow(t.Context(), `SELECT max_seen_at IS NOT NULL,(SELECT count(*) FROM video_cameras) FROM license_clock`).Scan(&observed, &cameras); err != nil || !observed || cameras != 3 {
		t.Fatalf("observed=%v cameras=%d error=%v", observed, cameras, err)
	}
}

func TestConfigure_whenTargetOutsideScope_precedesClockFailure(t *testing.T) {
	for _, item := range []struct {
		name     string
		id, pond int64
	}{{"foreign pond", 0, 103}, {"foreign camera", 103, 101}, {"missing camera", 999, 101}} {
		t.Run(item.name, func(t *testing.T) {
			// Given
			f := newFixture(t)
			exec(t, f.pool, `UPDATE license_clock SET clock_error=true`)
			// When
			var err error
			if item.id == 0 {
				_, err = f.service.Create(actor(1, "owner"), configuration(t, item.pond, false))
			} else {
				_, err = f.service.Replace(actor(1, "owner"), item.id, configuration(t, item.pond, false))
			}
			// Then
			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
