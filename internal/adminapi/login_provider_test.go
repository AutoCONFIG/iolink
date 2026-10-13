package adminapi

import (
	"context"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

type failedLoginLookup struct{ *fakeStore }

func (failedLoginLookup) FindAdminByLogin(context.Context, string) (*domain.User, error) {
	return nil, poisonAdminProviderError
}

type failedLegacyUpgrade struct{ *fakeStore }

func (failedLegacyUpgrade) UpgradeAdminPassword(context.Context, int64, string) error {
	return poisonAdminProviderError
}

func TestBaselineLoginProviderFailuresUseSafeDiagnostics(t *testing.T) {
	for _, tc := range []adminProviderHTTPCase{
		{"login_lookup", "POST", "/login", `{"username":"admin","password":"admin123"}`, func(f *fakeStore) AdminStore { return failedLoginLookup{f} }},
		{"login_legacy_upgrade", "POST", "/login", `{"username":"admin","password":"admin123"}`, func(f *fakeStore) AdminStore {
			legacy := "1cd663ce3300b9f52a357c4ae4e114064b0fa066071728aca1d7a98f5916f2e0"
			f.admin.PasswordHash = &legacy
			return failedLegacyUpgrade{f}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { exerciseAdminProviderFailure(t, tc) })
	}
}
