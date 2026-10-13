package core_test

import "testing"

func (f *keyOpenFixture) restore(t *testing.T, path string, status int) {
	t.Helper()
	raw := f.admin(t, keyOpenRequest{"POST", "/devices/{device_no}/restore", path, "", status})
	assertHTTPContract(t, "license", "POST", "/devices/{device_no}/restore", f.responseStatus, f.responseContentType, raw)
}

func TestKeyOpenContractRestoreRejectsForeignMissingAndReadOnly(t *testing.T) {
	f := newKeyOpenFixture(t)
	execute(t, f.pool, `UPDATE devices SET disabled_at=now(),status='offline'`)
	for _, tc := range []struct{ name, path string }{
		{"foreign", "/devices/m6d-foreign/restore"},
		{"missing", "/devices/missing/restore"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.state(t)
			f.restore(t, tc.path, 404)
			if f.state(t) != before {
				t.Error("rejected restore mutated state")
			}
		})
	}
	execute(t, f.pool, `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE user_id=8301`)
	var login struct {
		Token string `json:"token"`
	}
	m6bHTTPRequest(t, f.server.URL, "", "POST", "/user/v1/login", `{"username":"key-owner","password":"Key-open-only-1234"}`, 200, &login)
	f.token = login.Token
	t.Run("read_only", func(t *testing.T) {
		before := f.state(t)
		f.restore(t, "/devices/m6d-allowed/restore", 403)
		if f.state(t) != before {
			t.Error("read-only restore mutated state")
		}
	})
}

func TestKeyOpenContractRestoreLateStorageFailureRollsBackAndDiagnoses(t *testing.T) {
	f := newKeyOpenFixture(t)
	execute(t, f.pool, `UPDATE devices SET disabled_at=now(),status='offline' WHERE device_no='m6d-allowed'`)
	before := f.state(t)
	execute(t, f.pool, "ALTER TABLE device_shadows RENAME TO restore_contract_shadow_unavailable")
	f.restore(t, "/devices/m6d-allowed/restore", 500)
	execute(t, f.pool, "ALTER TABLE restore_contract_shadow_unavailable RENAME TO device_shadows")
	if f.state(t) != before {
		t.Error("late storage failure left device restoration committed")
	}
	f.databaseDiagnostic(t, "unused-fixture-secret")
}
