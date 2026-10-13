package core_test

import "testing"

func TestBaselineAdminRejectsMalformedQueryEncoding(t *testing.T) {
	f := newBaselineAdminFixture(t)
	for _, tc := range []baselineAdminRequest{
		{"GET", "/users", "/users?query=%zz", "", 400},
		{"GET", "/farms", "/farms?limit=%zz", "", 400},
		{"GET", "/ponds", "/ponds?farm_id=%zz", "", 400},
		{"GET", "/devices", "/devices?page=%zz", "", 400},
		{"GET", "/alarm-rules", "/alarm-rules?pond_id=%zz", "", 400},
		{"GET", "/alarms", "/alarms?limit=%zz", "", 400},
	} {
		t.Run(tc.route, func(t *testing.T) { f.request(t, tc) })
	}
}
