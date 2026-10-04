package domain

import (
	"context"
	"time"
)

type (
	tenantContextKey        struct{}
	tenantRoleContextKey    struct{}
	tenantUserContextKey    struct{}
	tenantVersionContextKey struct{}
)

type PermissionPolicy interface {
	Allow(role, resource, action string) (bool, error)
}

func WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

func TenantID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(tenantContextKey{}).(int64)
	return id, ok && id > 0
}

func WithTenantRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, tenantRoleContextKey{}, role)
}

func TenantRole(ctx context.Context) string {
	role, _ := ctx.Value(tenantRoleContextKey{}).(string)
	return role
}

func WithTenantUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, tenantUserContextKey{}, userID)
}

func TenantUserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(tenantUserContextKey{}).(int64)
	return id, ok && id > 0
}

func HasTenantScope(ctx context.Context) bool {
	return ctx.Value(tenantContextKey{}) != nil || ctx.Value(tenantRoleContextKey{}) != nil || ctx.Value(tenantUserContextKey{}) != nil || ctx.Value(tenantVersionContextKey{}) != nil
}

func WithTenantPermissionVersion(ctx context.Context, version int64) context.Context {
	return context.WithValue(ctx, tenantVersionContextKey{}, version)
}

func TenantPermissionVersion(ctx context.Context) (int64, bool) {
	version, ok := ctx.Value(tenantVersionContextKey{}).(int64)
	return version, ok
}

type TenantMembership struct {
	TenantID  int64      `json:"tenant_id"`
	UserID    int64      `json:"user_id"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	Active    bool       `json:"active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Tenant struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Active            bool   `json:"active"`
	PermissionVersion int64  `json:"permission_version"`
}

type FarmMembership struct {
	TenantID  int64      `json:"tenant_id"`
	FarmID    int64      `json:"farm_id"`
	UserID    int64      `json:"user_id"`
	Role      string     `json:"role"`
	Active    bool       `json:"active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
