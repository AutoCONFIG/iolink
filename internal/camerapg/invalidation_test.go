package camerapg_test

import (
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestReplace_whenOldSessionExists(t *testing.T) {
	// Given
	f := newFixture(t)
	addSession(t, f.pool)
	// When
	got, err := f.service.Replace(actor(1, "owner"), 101, configuration(t, 102, true))
	// Then
	if err != nil || got.SourceVersion != 2 || got.PondID != 102 || got.Status != "offline" {
		t.Fatalf("camera=%+v error=%v", got, err)
	}
	var state string
	var revoked bool
	var jobs, history int
	if err = f.pool.QueryRow(t.Context(), `SELECT state,revoked_at IS NOT NULL,(SELECT count(*) FROM jobs WHERE kind='video.stop'),(SELECT count(*) FROM video_streams) FROM video_sessions`).Scan(&state, &revoked, &jobs, &history); err != nil {
		t.Fatal(err)
	}
	if state != "revoked" || !revoked || jobs != 1 || history != 1 {
		t.Fatal("old sessions/history/stop intent incorrect")
	}
	var safe bool
	if err = f.pool.QueryRow(t.Context(), `SELECT payload=jsonb_build_object('camera_id',101,'source_version',1,'stream_id','00000000-0000-4000-8000-000000000101','action','stop') FROM jobs`).Scan(&safe); err != nil || !safe {
		t.Fatal("unsafe stop job payload")
	}
}

func TestDisable_whenLicenseAndRuntimeUnavailable(t *testing.T) {
	// Given
	f := newFixture(t)
	addSession(t, f.pool)
	f.deps.License = nil
	f.deps.Cipher = nil
	f.deps.Availability = nil
	s := camera.New(f.deps)
	// When
	err := s.Disable(actor(1, "owner"), 101)
	// Then
	if err != nil {
		t.Fatal(err)
	}
	var enabled bool
	var audit, jobs, cameras, sessions int
	if err = f.pool.QueryRow(t.Context(), `SELECT enabled,(SELECT count(*) FROM audit_events WHERE action='camera.disabled'),(SELECT count(*) FROM jobs),(SELECT count(*) FROM video_cameras),(SELECT count(*) FROM video_sessions) FROM video_cameras WHERE id=101`).Scan(&enabled, &audit, &jobs, &cameras, &sessions); err != nil {
		t.Fatal(err)
	}
	if enabled || audit != 1 || jobs != 1 || cameras != 3 || sessions != 1 {
		t.Fatal("disable did not retain history and atomic intents")
	}
	if _, err = s.Get(actor(1, "owner"), 101); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("disabled camera visible: %v", err)
	}
}

func TestDisable_whenAlreadyDisabled(t *testing.T) {
	// Given
	f := newFixture(t)
	addSession(t, f.pool)
	if err := f.service.Disable(actor(1, "owner"), 101); err != nil {
		t.Fatal(err)
	}
	// When
	err := f.service.Disable(actor(1, "owner"), 101)
	// Then
	if err != nil {
		t.Fatal(err)
	}
	var audit, jobs int
	if err = f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM audit_events WHERE action='camera.disabled'),(SELECT count(*) FROM jobs)`).Scan(&audit, &jobs); err != nil || audit != 1 || jobs != 1 {
		t.Fatalf("audit=%d jobs=%d error=%v", audit, jobs, err)
	}
}

func TestReplace_whenAnonymousConfigurationSubmitted(t *testing.T) {
	// Given
	f := newFixture(t)
	got, err := f.service.Create(actor(1, "owner"), configuration(t, 101, true))
	if err != nil {
		t.Fatal(err)
	}
	// When
	_, err = f.service.Replace(actor(1, "owner"), got.ID, configuration(t, 101, false))
	// Then
	if err != nil {
		t.Fatal(err)
	}
	var anonymous bool
	var version int64
	if err = f.pool.QueryRow(t.Context(), `SELECT credential_cipher IS NULL,source_version FROM video_cameras WHERE id=$1`, got.ID).Scan(&anonymous, &version); err != nil || !anonymous || version != 2 {
		t.Fatalf("anonymous=%v version=%d error=%v", anonymous, version, err)
	}
}

func TestMutation_whenAuditOrJobInsertFails(t *testing.T) {
	for _, table := range []string{"audit_events", "jobs"} {
		t.Run(table, func(t *testing.T) {
			// Given
			f := newFixture(t)
			addSession(t, f.pool)
			exec(t, f.pool, `CREATE FUNCTION reject_camera_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'secret provider error rtsp://private-host credential'; END $$; CREATE TRIGGER reject_camera BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION reject_camera_mutation()`)
			// When
			_, err := f.service.Replace(actor(1, "owner"), 101, configuration(t, 102, true))
			// Then
			if !errors.Is(err, camera.ErrInternal) || err.Error() != camera.ErrInternal.Error() {
				t.Fatalf("unsafe error=%v", err)
			}
			var version, pond int64
			var session string
			var jobs, audits int
			if err = f.pool.QueryRow(t.Context(), `SELECT source_version,pond_id,(SELECT state FROM video_sessions),(SELECT count(*) FROM jobs),(SELECT count(*) FROM audit_events WHERE resource_type='camera') FROM video_cameras WHERE id=101`).Scan(&version, &pond, &session, &jobs, &audits); err != nil {
				t.Fatal(err)
			}
			if version != 1 || pond != 101 || session != "ready" || jobs != 0 || audits != 0 {
				t.Fatal("failed mutation left partial state")
			}
		})
	}
}
