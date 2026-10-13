package adminapi

import (
	"context"
	"errors"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

var poisonAdminProviderError = errors.New("postgres://private-user:private-password@private-host/private-db?token=private-token")

type failfarmList struct{ *fakeStore }

func (failfarmList) ListFarms(context.Context) ([]domain.Farm, error) {
	return nil, poisonAdminProviderError
}

type failfarmCreate struct{ *fakeStore }

func (failfarmCreate) CreateFarm(context.Context, *int64, string, string) (domain.Farm, error) {
	return domain.Farm{}, poisonAdminProviderError
}

type failfarmUpdate struct{ *fakeStore }

func (failfarmUpdate) UpdateFarm(context.Context, int64, string, string) error {
	return poisonAdminProviderError
}

type failfarmDelete struct{ *fakeStore }

func (failfarmDelete) DeleteFarm(context.Context, int64) error { return poisonAdminProviderError }

type failfarmOwner struct{ *fakeStore }

func (failfarmOwner) SetFarmOwner(context.Context, int64, *int64) error {
	return poisonAdminProviderError
}

type failfarmOwnerActor struct{ *fakeStore }

func (failfarmOwnerActor) SetFarmOwnerByActor(context.Context, int64, *int64, int64) error {
	return poisonAdminProviderError
}

type failuserSearch struct{ *fakeStore }

func (failuserSearch) SearchUsers(context.Context, string, int, int) ([]domain.User, error) {
	return nil, poisonAdminProviderError
}

type failpondList struct{ *fakeStore }

func (failpondList) ListPonds(context.Context) ([]domain.Pond, error) {
	return nil, poisonAdminProviderError
}

type failpondFarm struct{ *fakeStore }

func (failpondFarm) FarmExists(context.Context, int64) (bool, error) {
	return false, poisonAdminProviderError
}

type failpondCreate struct{ *fakeStore }

func (failpondCreate) CreatePond(context.Context, int64, string, float64) (domain.Pond, error) {
	return domain.Pond{}, poisonAdminProviderError
}

type failpondUpdate struct{ *fakeStore }

func (failpondUpdate) UpdatePond(context.Context, int64, string, float64) error {
	return poisonAdminProviderError
}

func (failpondUpdate) UpdatePondInFarm(context.Context, int64, int64, string, float64) error {
	return poisonAdminProviderError
}

type failpondDelete struct{ *fakeStore }

func (failpondDelete) DeletePond(context.Context, int64) error { return poisonAdminProviderError }

type faildeviceRegister struct{ *fakeStore }

func (faildeviceRegister) RegisterDevice(context.Context, int64, string, string, int) (domain.Device, string, error) {
	return domain.Device{}, "", poisonAdminProviderError
}

type faildeviceList struct{ *fakeStore }

func (faildeviceList) ListDevicesPage(context.Context, domain.DeviceListQuery) (domain.DevicePage, error) {
	return domain.DevicePage{}, poisonAdminProviderError
}

type faildeviceMove struct{ *fakeStore }

func (faildeviceMove) MoveDevice(context.Context, string, int64) error {
	return poisonAdminProviderError
}

type faildeviceDelete struct{ *fakeStore }

func (faildeviceDelete) DeleteDevice(context.Context, string) error { return poisonAdminProviderError }

type faildeviceRestore struct{ *fakeStore }

func (faildeviceRestore) RestoreDevice(context.Context, string) error {
	return poisonAdminProviderError
}

type failruleList struct{ *fakeStore }

func (failruleList) ListRules(context.Context) ([]domain.AlarmRule, error) {
	return nil, poisonAdminProviderError
}

type failruleCreate struct{ *fakeStore }

func (failruleCreate) CreateRule(context.Context, domain.AlarmRule) (domain.AlarmRule, error) {
	return domain.AlarmRule{}, poisonAdminProviderError
}

type failruleUpdate struct{ *fakeStore }

func (failruleUpdate) UpdateRule(context.Context, domain.AlarmRule) error {
	return poisonAdminProviderError
}

type failruleDelete struct{ *fakeStore }

func (failruleDelete) DeleteRule(context.Context, int64) error { return poisonAdminProviderError }

type failalarmList struct{ *fakeStore }

func (failalarmList) ListAllAlarms(context.Context, int) ([]domain.Alarm, error) {
	return nil, poisonAdminProviderError
}

type failalarmConfirm struct{ *fakeStore }

func (failalarmConfirm) ConfirmAlarm(context.Context, int64) error { return poisonAdminProviderError }

type failalarmConfirmActor struct{ *fakeStore }

func (failalarmConfirmActor) ConfirmAlarmByActor(context.Context, int64, int64) error {
	return poisonAdminProviderError
}

type failalarmBatch struct{ *fakeStore }

func (failalarmBatch) BatchConfirm(context.Context, []int64) (int64, error) {
	return 0, poisonAdminProviderError
}

type failalarmBatchActor struct{ *fakeStore }

func (failalarmBatchActor) BatchConfirmByActor(context.Context, []int64, int64) (int64, error) {
	return 0, poisonAdminProviderError
}

type failstats struct{ *fakeStore }

func (failstats) Stats(context.Context) (domain.Stats, error) {
	return domain.Stats{}, poisonAdminProviderError
}

type failpassword struct{ *fakeStore }

func (failpassword) ChangeAdminPassword(context.Context, int64, string, string) error {
	return poisonAdminProviderError
}

type failloginVersion struct{ *fakeStore }

func (failloginVersion) AdminTokenVersion(context.Context, int64) (int, error) {
	return 0, poisonAdminProviderError
}

type failloginTenant struct{ *fakeStore }

func (failloginTenant) DefaultTenantForUser(context.Context, int64) (int64, error) {
	return 0, poisonAdminProviderError
}

type failloginMembership struct{ *fakeStore }

func (failloginMembership) TenantMembershipVersion(context.Context, int64, int64) (int64, error) {
	return 0, poisonAdminProviderError
}

type failloginRole struct{ *fakeStore }

func (failloginRole) TenantRole(context.Context, int64, int64) (string, error) {
	return "", poisonAdminProviderError
}

type failfarmMemberList struct{ *fakeStore }

func (failfarmMemberList) ListFarmMembers(context.Context, int64) ([]domain.FarmMembership, error) {
	return nil, poisonAdminProviderError
}

type failfarmMemberSet struct{ *fakeStore }

func (failfarmMemberSet) SetFarmMember(context.Context, int64, int64, string, bool, *time.Time, int64) error {
	return poisonAdminProviderError
}

func (failfarmMemberList) SetFarmMember(context.Context, int64, int64, string, bool, *time.Time, int64) error {
	return nil
}

func (failfarmMemberSet) ListFarmMembers(context.Context, int64) ([]domain.FarmMembership, error) {
	return nil, nil
}
