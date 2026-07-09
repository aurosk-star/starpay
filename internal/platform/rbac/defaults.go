package rbac

import "github.com/casbin/casbin/v2"

func LoadDefaultPolicies(enforcer *casbin.Enforcer) error {
	policies := [][]string{
		{"super_admin", "/v1/admin/*", "*"},
		{"super_admin", "/v1/admin/apps", "*"},
		{"super_admin", "/v1/admin/apps/*", "*"},
		{"super_admin", "/v1/admin/channels", "*"},
		{"super_admin", "/v1/admin/channels/*", "*"},
		{"super_admin", "/v1/admin/orders", "*"},
		{"super_admin", "/v1/admin/orders/*", "*"},
		{"operator", "/v1/admin/auth/me", "GET"},
		{"operator", "/v1/admin/users", "GET"},
		{"operator", "/v1/admin/roles", "GET"},
		{"operator", "/v1/admin/apps", "GET"},
		{"operator", "/v1/admin/channels", "GET"},
		{"operator", "/v1/admin/routing-rules", "GET"},
		{"operator", "/v1/admin/routing-rules/*", "GET"},
		{"operator", "/v1/admin/orders", "GET"},
		{"operator", "/v1/admin/orders/*", "GET"},
		{"operator", "/v1/admin/monitoring/overview", "GET"},
		{"viewer", "/v1/admin/auth/me", "GET"},
		{"viewer", "/v1/admin/users", "GET"},
		{"viewer", "/v1/admin/roles", "GET"},
		{"viewer", "/v1/admin/apps", "GET"},
		{"viewer", "/v1/admin/channels", "GET"},
		{"viewer", "/v1/admin/routing-rules", "GET"},
		{"viewer", "/v1/admin/routing-rules/*", "GET"},
		{"viewer", "/v1/admin/orders", "GET"},
		{"viewer", "/v1/admin/orders/*", "GET"},
		{"viewer", "/v1/admin/monitoring/overview", "GET"},
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
