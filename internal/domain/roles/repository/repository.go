package repository

import (
	"context"

	"payment-gateway/ent"
	"payment-gateway/ent/role"
)

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

func (r Repository) EnsureDefaults(ctx context.Context) ([]*ent.Role, error) {
	defaults := []struct {
		Code        string
		Name        string
		Description string
	}{
		{"super_admin", "超级管理员", "拥有全部后台权限"},
		{"operator", "运营人员", "可处理订单、退款、Webhook 与补偿任务"},
		{"viewer", "只读人员", "只能查看后台数据"},
	}

	roles := make([]*ent.Role, 0, len(defaults))
	for _, item := range defaults {
		existing, err := r.client.Role.Query().Where(role.Code(item.Code)).Only(ctx)
		if err == nil {
			roles = append(roles, existing)
			continue
		}
		if !ent.IsNotFound(err) {
			return nil, err
		}
		created, err := r.client.Role.Create().
			SetCode(item.Code).
			SetName(item.Name).
			SetDescription(item.Description).
			SetStatus("enabled").
			Save(ctx)
		if err != nil {
			return nil, err
		}
		roles = append(roles, created)
	}
	return roles, nil
}

func (r Repository) FindByCode(ctx context.Context, code string) (*ent.Role, error) {
	return r.client.Role.Query().Where(role.Code(code)).Only(ctx)
}

func (r Repository) List(ctx context.Context) ([]*ent.Role, error) {
	return r.client.Role.Query().Order(ent.Asc(role.FieldID)).All(ctx)
}
