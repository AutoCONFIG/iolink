package authorization

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const policyModel = `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p_eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && (r.act == p.act || p.act == "*")`

type Policy struct {
	enforcer *casbin.SyncedEnforcer
}

func New() (*Policy, error) {
	m, err := model.NewModelFromString(policyModel)
	if err != nil {
		return nil, err
	}
	e, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, err
	}
	for _, role := range []string{"owner", "admin"} {
		for _, resource := range []string{"farms", "farm_members", "users", "ponds", "devices", "alarm_rules", "alarms", "stats", "products", "tenant_members", "telemetry"} {
			if _, err := e.AddPolicy(role, resource, "*"); err != nil {
				return nil, err
			}
		}
	}
	for _, role := range []string{"member", "viewer", "support"} {
		for _, resource := range []string{"farms", "ponds", "devices", "alarm_rules", "alarms", "stats", "products"} {
			if _, err := e.AddPolicy(role, resource, "read"); err != nil {
				return nil, err
			}
		}
	}
	for _, role := range []string{"member", "support"} {
		if _, err := e.AddPolicy(role, "alarms", "confirm"); err != nil {
			return nil, err
		}
	}
	return &Policy{enforcer: e}, nil
}

func (p *Policy) Allow(role, resource, action string) (bool, error) {
	return p.enforcer.Enforce(role, resource, action)
}
