package rbac

import "github.com/casbin/casbin/v2"

func LoadDefaultPolicies(enforcer *casbin.Enforcer) error {
	policies := [][]string{
		{"super_admin", "/v1/admin/*", "*"},
		{"operator", "/v1/admin/auth/me", "GET"},
		{"operator", "/v1/admin/users", "GET"},
		{"operator", "/v1/admin/roles", "GET"},
		{"viewer", "/v1/admin/auth/me", "GET"},
		{"viewer", "/v1/admin/users", "GET"},
		{"viewer", "/v1/admin/roles", "GET"},
	}
	for _, policy := range policies {
		if _, err := enforcer.AddPolicy(policy); err != nil {
			return err
		}
	}
	for _, role := range []string{"super_admin", "operator", "viewer"} {
		if _, err := enforcer.AddGroupingPolicy(role, role); err != nil {
			return err
		}
	}
	return nil
}
