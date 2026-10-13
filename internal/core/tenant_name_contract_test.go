package core_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTenantNameLimitCountsUnicodeCharacters(t *testing.T) {
	f := newPlatformContractFixture(t)
	for _, tc := range []struct {
		name   string
		status int
	}{{strings.Repeat("界", 128), 201}, {strings.Repeat("界", 129), 400}} {
		t.Run("name_length", func(t *testing.T) {
			body, err := json.Marshal(struct {
				Name string `json:"name"`
			}{tc.name})
			if err != nil {
				t.Fatal(err)
			}
			before := count(t, f.pool, "SELECT count(*) FROM tenants")
			f.request(t, platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.admin, string(body), tc.status})
			want := before
			if tc.status == 201 {
				want++
			}
			if count(t, f.pool, "SELECT count(*) FROM tenants") != want {
				t.Error("tenant name acceptance and persisted count differ")
			}
		})
	}
}
