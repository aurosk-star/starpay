package repository

import (
	"context"

	"payment-gateway/ent"
	"payment-gateway/ent/role"
	"payment-gateway/ent/user"
)

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

func (r Repository) HasSuperAdmin(ctx context.Context) (bool, error) {
	return r.client.User.Query().
		Where(user.HasRolesWith(role.Code("super_admin"))).
		Exist(ctx)
}

func (r Repository) FindByUsername(ctx context.Context, username string) (*ent.User, error) {
	return r.client.User.Query().
		Where(user.Username(username)).
		WithRoles().
		Only(ctx)
}

func (r Repository) FindByID(ctx context.Context, id int) (*ent.User, error) {
	return r.client.User.Query().
		Where(user.ID(id)).
		WithRoles().
		Only(ctx)
}

func (r Repository) List(ctx context.Context) ([]*ent.User, error) {
	return r.client.User.Query().
		WithRoles().
		Order(ent.Desc(user.FieldCreatedAt)).
		All(ctx)
}

func (r Repository) Create(ctx context.Context, input CreateUserInput) (*ent.User, error) {
	create := r.client.User.Create().
		SetUsername(input.Username).
		SetEmail(input.Email).
		SetPasswordHash(input.PasswordHash).
		SetStatus("enabled")
	if input.DisplayName != "" {
		create.SetDisplayName(input.DisplayName)
	}
	if len(input.RoleIDs) > 0 {
		create.AddRoleIDs(input.RoleIDs...)
	}
	return create.Save(ctx)
}

func (r Repository) SetRoles(ctx context.Context, userID int, roleIDs []int) error {
	_, err := r.client.User.UpdateOneID(userID).ClearRoles().AddRoleIDs(roleIDs...).Save(ctx)
	return err
}

type CreateUserInput struct {
	Username     string
	Email        string
	DisplayName  string
	PasswordHash string
	RoleIDs      []int
}
