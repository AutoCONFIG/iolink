package core_test

import "testing"

func TestBaselineAdminLoginRejectsInvalidInputAndClassifiesStorageFailure(t *testing.T) {
	// Given: a real account and production HTTP routes backed by isolated PostgreSQL.
	f := newBaselineAdminFixture(t)
	for _, body := range []string{
		`{"username":"contract-owner","password":"Contract-test-only-1234","extra":true}`,
		`{"username":"contract-owner","password":"Contract-test-only-1234"} {}`,
		`{"username":"contract-owner","username":"other","password":"Contract-test-only-1234"}`,
		`{"Username":"contract-owner","password":"Contract-test-only-1234"}`,
		`{"username":null,"password":"Contract-test-only-1234"}`,
		`{"username":"contract-owner","password":null}`,
	} {
		t.Run("invalid_input", func(t *testing.T) {
			// When: invalid wire objects are sent to the actual login boundary.
			f.request(t, baselineAdminRequest{"POST", "/login", "/login", body, 400})
			// Then: neither credentials nor their revocation version change.
			if count(t, f.pool, `SELECT count(*) FROM users WHERE id=1 AND token_version=0`) != 1 {
				t.Error("invalid login changed token version")
			}
		})
	}
	t.Run("unknown_identity", func(t *testing.T) {
		f.request(t, baselineAdminRequest{"POST", "/login", "/login", `{"username":"absent","password":"Contract-test-only-1234"}`, 401})
	})
	t.Run("real_storage_failure", func(t *testing.T) {
		execute(t, f.pool, "ALTER TABLE users RENAME TO login_fault_relation")
		defer execute(t, f.pool, "ALTER TABLE login_fault_relation RENAME TO users")
		f.request(t, baselineAdminRequest{"POST", "/login", "/login", `{"username":"contract-owner","password":"Contract-test-only-1234"}`, 500})
	})
	t.Run("recovered_identity", func(t *testing.T) {
		f.request(t, baselineAdminRequest{"POST", "/login", "/login", `{"username":"contract-owner","password":"Contract-test-only-1234"}`, 200})
	})
}
