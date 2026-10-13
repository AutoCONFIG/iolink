package core_test

import "testing"

func TestBaselineNumericFiltersRejectExplicitEmptyValues(t *testing.T) {
	f := newBaselineAdminFixture(t)
	for _, tc := range []baselineAdminRequest{
		{"GET", "/ponds", "/ponds?farm_id=", "", 400},
		{"GET", "/alarm-rules", "/alarm-rules?pond_id=", "", 400},
	} {
		t.Run(tc.route, func(t *testing.T) { f.request(t, tc) })
	}
}
