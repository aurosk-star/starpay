package userstest

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/config"
)

func TestCreateUserAssignsRolesAndHashesPassword(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_user?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := usersvc.New(client, testAuthConfig())
	roles, err := svc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}

	operatorID := roleID(t, roles, "operator")
	created, err := svc.CreateUser(ctx, usersvc.ManageUserInput{
		Username:    "ops",
		Email:       "ops@example.com",
		Password:    "password123",
		DisplayName: "运营",
		Status:      "enabled",
		RoleIDs:     []int{operatorID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if created.PasswordHash == "password123" {
		t.Fatal("password was stored without hashing")
	}
	if len(created.Edges.Roles) != 1 || created.Edges.Roles[0].Code != "operator" {
		t.Fatalf("roles = %#v, want operator", created.Edges.Roles)
	}
}

func TestUpdateUserChangesProfileStatusAndRoles(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:update_user?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := usersvc.New(client, testAuthConfig())
	roles, err := svc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	viewerID := roleID(t, roles, "viewer")
	operatorID := roleID(t, roles, "operator")

	created, err := svc.CreateUser(ctx, usersvc.ManageUserInput{
		Username: "auditor",
		Email:    "audit@example.com",
		Password: "password123",
		Status:   "enabled",
		RoleIDs:  []int{viewerID},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	updated, err := svc.UpdateUser(ctx, created.ID, usersvc.ManageUserInput{
		Username:    "auditor2",
		Email:       "audit2@example.com",
		DisplayName: "审计员",
		Status:      "disabled",
		RoleIDs:     []int{operatorID},
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	if updated.Username != "auditor2" || updated.Email != "audit2@example.com" || updated.Status != "disabled" {
		t.Fatalf("updated user = %#v", updated)
	}
	if len(updated.Edges.Roles) != 1 || updated.Edges.Roles[0].Code != "operator" {
		t.Fatalf("roles = %#v, want operator", updated.Edges.Roles)
	}
}

func TestDeleteUserDisablesUser(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:disable_user?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := usersvc.New(client, testAuthConfig())
	roles, err := svc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}

	created, err := svc.CreateUser(ctx, usersvc.ManageUserInput{
		Username: "temp",
		Email:    "temp@example.com",
		Password: "password123",
		Status:   "enabled",
		RoleIDs:  []int{roleID(t, roles, "viewer")},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if err := svc.DeleteUser(ctx, created.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	found, err := svc.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.Status != "disabled" {
		t.Fatalf("Status = %q, want disabled", found.Status)
	}
}

func roleID(t *testing.T, roles []*ent.Role, code string) int {
	t.Helper()
	for _, item := range roles {
		if item.Code == code {
			return item.ID
		}
	}
	t.Fatalf("role %q not found", code)
	return 0
}

func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}
}
