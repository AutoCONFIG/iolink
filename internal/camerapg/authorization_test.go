package camerapg_test

import (
	"context"
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestRead_whenRoleAndFarmScopeVary(t *testing.T) {
	for _, item := range []struct {
		role   string
		id     int64
		count  int
		denied bool
	}{
		{"owner", 1, 2, false}, {"admin", 2, 2, false}, {"member", 3, 1, false}, {"viewer", 4, 1, false}, {"support", 5, 1, false}, {"member", 6, 0, true}, {"member", 7, 1, false},
	} {
		t.Run(item.role+string(rune('0'+item.id)), func(t *testing.T) {
			// Given
			f := newFixture(t)
			// When
			list, err := f.service.List(actor(item.id, item.role), camera.Page{})
			// Then
			if item.denied {
				if !errors.Is(err, domain.ErrForbidden) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil || len(list.Items) != item.count {
				t.Fatalf("items=%d error=%v", len(list.Items), err)
			}
			for _, c := range list.Items {
				if c.ID == 103 {
					t.Fatal("cross tenant camera disclosed")
				}
			}
		})
	}
}

func TestConfigure_whenRoleLacksWritePermission(t *testing.T) {
	for _, item := range []struct {
		role string
		id   int64
	}{{"member", 3}, {"viewer", 4}, {"support", 5}, {"member", 6}} {
		t.Run(item.role+string(rune('0'+item.id)), func(t *testing.T) {
			// Given
			f := newFixture(t)
			// When
			_, err := f.service.Create(actor(item.id, item.role), configuration(t, 101, false))
			// Then
			if !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRead_whenContextOrMembershipIsInvalid(t *testing.T) {
	for _, item := range []struct {
		name string
		ctx  context.Context
		sql  string
	}{
		{"missing tenant", context.Background(), ""},
		{"forged role", actor(3, "owner"), ""},
		{"stale permission", domain.WithTenantPermissionVersion(actor(1, "owner"), 9), ""},
		{"inactive member", actor(1, "owner"), `UPDATE tenant_memberships SET active=false WHERE user_id=1`},
		{"inactive tenant", actor(1, "owner"), `UPDATE tenants SET active=false WHERE id=101`},
		{"expired member", actor(3, "member"), `UPDATE tenant_memberships SET expires_at=now()-interval '1 second' WHERE user_id=3`},
		{"expired support", actor(5, "support"), `UPDATE tenant_memberships SET expires_at=now()-interval '1 second' WHERE user_id=5`},
	} {
		t.Run(item.name, func(t *testing.T) {
			// Given
			f := newFixture(t)
			if item.sql != "" {
				exec(t, f.pool, item.sql)
			}
			// When
			_, err := f.service.Get(item.ctx, 101)
			// Then
			if !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestGet_whenResourceOutsideCurrentFarm(t *testing.T) {
	for _, item := range []struct {
		name string
		id   int64
		sql  string
	}{{"same tenant other farm", 102, ""}, {"foreign tenant", 103, ""}, {"missing", 999, ""}, {"revoked farm", 101, `UPDATE farm_memberships SET active=false WHERE user_id=3`}, {"expired farm", 101, `UPDATE farm_memberships SET expires_at=now()-interval '1 second' WHERE user_id=3`}} {
		t.Run(item.name, func(t *testing.T) {
			// Given
			f := newFixture(t)
			if item.sql != "" {
				exec(t, f.pool, item.sql)
			}
			// When
			_, err := f.service.Get(actor(3, "member"), item.id)
			// Then
			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestCreate_whenPondBelongsToDifferentTenant(t *testing.T) {
	// Given
	f := newFixture(t)
	// When
	_, err := f.service.Create(actor(1, "owner"), configuration(t, 103, false))
	// Then
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
	var count int
	if err = f.pool.QueryRow(t.Context(), `SELECT count(*) FROM video_cameras`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("count=%d error=%v", count, err)
	}
}

func TestList_whenCursorAndLimitUsed(t *testing.T) {
	// Given
	f := newFixture(t)
	// When
	list, err := f.service.List(actor(1, "owner"), camera.Page{Limit: 1})
	// Then
	if err != nil || len(list.Items) != 1 || list.Items[0].ID != 101 || list.NextAfterID == nil || *list.NextAfterID != 101 {
		t.Fatalf("list=%+v error=%v", list, err)
	}
}
