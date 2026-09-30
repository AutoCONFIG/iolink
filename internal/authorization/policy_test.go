package authorization

import "testing"

func TestTenantRoleResourceActionPolicy(t *testing.T) {
	policy, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		role, resource, action string
		want                   bool
	}{
		{"owner", "farm_members", "read", true},
		{"admin", "tenant_members", "write", true},
		{"member", "alarms", "confirm", true},
		{"support", "alarms", "confirm", true},
		{"viewer", "alarms", "confirm", false},
		{"support", "farm_members", "read", false},
		{"member", "tenant_members", "read", false},
		{"viewer", "products", "write", false},
		{"platform", "farms", "read", false},
		{"owner", "unregistered_resource", "read", false},
	} {
		t.Run(tc.role+"/"+tc.resource+"/"+tc.action, func(t *testing.T) {
			got, err := policy.Allow(tc.role, tc.resource, tc.action)
			if err != nil || got != tc.want {
				t.Fatalf("allow=%t want=%t err=%v", got, tc.want, err)
			}
		})
	}
}
