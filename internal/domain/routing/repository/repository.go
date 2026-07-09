package repository

import (
	"context"

	"payment-gateway/ent"
	"payment-gateway/ent/routingrule"
	"payment-gateway/ent/routingtarget"
)

type Repository struct {
	client *ent.Client
}

func New(client *ent.Client) Repository {
	return Repository{client: client}
}

func (r Repository) IsZero() bool {
	return r.client == nil
}

type RuleAggregate struct {
	Rule    *ent.RoutingRule
	Targets []*ent.RoutingTarget
}

type RuleMutation struct {
	Name          string
	Enabled       bool
	Priority      int
	AppScope      string
	AppIDs        []string
	PaymentMethod string
	PayModes      []string
	Currency      string
	MinAmount     int64
	MaxAmount     int64
	Terminal      string
	Metadata      map[string]any
	Targets       []TargetMutation
}

type TargetMutation struct {
	ChannelAccountID int
	Enabled          bool
	Priority         int
	Weight           int
}

func (r Repository) List(ctx context.Context) ([]RuleAggregate, error) {
	rules, err := r.client.RoutingRule.Query().
		Order(ent.Desc(routingrule.FieldPriority), ent.Asc(routingrule.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]RuleAggregate, 0, len(rules))
	for _, rule := range rules {
		targets, err := r.targetsForRule(ctx, rule.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, RuleAggregate{Rule: rule, Targets: targets})
	}
	return items, nil
}

func (r Repository) FindByID(ctx context.Context, id int) (*RuleAggregate, error) {
	rule, err := r.client.RoutingRule.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	targets, err := r.targetsForRule(ctx, rule.ID)
	if err != nil {
		return nil, err
	}
	return &RuleAggregate{Rule: rule, Targets: targets}, nil
}

func (r Repository) Create(ctx context.Context, input RuleMutation) (*RuleAggregate, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	rule, err := tx.RoutingRule.Create().
		SetName(input.Name).
		SetEnabled(input.Enabled).
		SetPriority(input.Priority).
		SetAppScope(input.AppScope).
		SetAppIds(input.AppIDs).
		SetPaymentMethod(input.PaymentMethod).
		SetPayModes(input.PayModes).
		SetCurrency(input.Currency).
		SetMinAmount(input.MinAmount).
		SetMaxAmount(input.MaxAmount).
		SetTerminal(input.Terminal).
		SetMetadata(input.Metadata).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	targets, err := createTargets(ctx, tx, rule.ID, input.Targets)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &RuleAggregate{Rule: rule, Targets: targets}, nil
}

func (r Repository) Update(ctx context.Context, id int, input RuleMutation) (*RuleAggregate, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	rule, err := tx.RoutingRule.UpdateOneID(id).
		SetName(input.Name).
		SetEnabled(input.Enabled).
		SetPriority(input.Priority).
		SetAppScope(input.AppScope).
		SetAppIds(input.AppIDs).
		SetPaymentMethod(input.PaymentMethod).
		SetPayModes(input.PayModes).
		SetCurrency(input.Currency).
		SetMinAmount(input.MinAmount).
		SetMaxAmount(input.MaxAmount).
		SetTerminal(input.Terminal).
		SetMetadata(input.Metadata).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if _, err := tx.RoutingTarget.Delete().
		Where(routingtarget.RoutingRuleID(id)).
		Exec(ctx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	targets, err := createTargets(ctx, tx, rule.ID, input.Targets)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &RuleAggregate{Rule: rule, Targets: targets}, nil
}

func (r Repository) SetEnabled(ctx context.Context, id int, enabled bool) (*RuleAggregate, error) {
	if _, err := r.client.RoutingRule.UpdateOneID(id).SetEnabled(enabled).Save(ctx); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r Repository) targetsForRule(ctx context.Context, ruleID int) ([]*ent.RoutingTarget, error) {
	return r.client.RoutingTarget.Query().
		Where(routingtarget.RoutingRuleID(ruleID)).
		Order(ent.Desc(routingtarget.FieldPriority), ent.Asc(routingtarget.FieldID)).
		All(ctx)
}

func createTargets(ctx context.Context, tx *ent.Tx, ruleID int, inputs []TargetMutation) ([]*ent.RoutingTarget, error) {
	targets := make([]*ent.RoutingTarget, 0, len(inputs))
	for _, input := range inputs {
		target, err := tx.RoutingTarget.Create().
			SetRoutingRuleID(ruleID).
			SetChannelAccountID(input.ChannelAccountID).
			SetEnabled(input.Enabled).
			SetPriority(input.Priority).
			SetWeight(input.Weight).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}
