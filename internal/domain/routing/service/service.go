package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"payment-gateway/ent"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	paymentsvc "payment-gateway/internal/domain/payments/service"
	routingrepo "payment-gateway/internal/domain/routing/repository"
)

const (
	AppScopeAll     = "all"
	AppScopeInclude = "include"
)

var (
	ErrRuleNameRequired              = errors.New("routing rule name is required")
	ErrInvalidPaymentMethod          = errors.New("invalid routing payment method")
	ErrInvalidAppScope               = errors.New("invalid routing app scope")
	ErrInvalidTerminal               = errors.New("invalid routing terminal")
	ErrInvalidAmountRange            = errors.New("invalid routing amount range")
	ErrRoutingTargetRequired         = errors.New("routing target is required")
	ErrInvalidTargetChannelAccountID = errors.New("invalid routing target channel account id")
)

type Service struct {
	rules    routingrepo.Repository
	channels channelrepo.Repository
}

func New(client *ent.Client) Service {
	return Service{
		rules:    routingrepo.New(client),
		channels: channelrepo.New(client),
	}
}

func (s Service) IsZero() bool {
	return s.rules.IsZero()
}

type ManageRuleInput struct {
	Name          string              `json:"name"`
	Enabled       bool                `json:"enabled"`
	Priority      int                 `json:"priority"`
	AppScope      string              `json:"app_scope"`
	AppIDs        []string            `json:"app_ids"`
	PaymentMethod string              `json:"payment_method"`
	PayModes      []string            `json:"pay_modes"`
	Currency      string              `json:"currency"`
	MinAmount     int64               `json:"min_amount"`
	MaxAmount     int64               `json:"max_amount"`
	Terminal      string              `json:"terminal"`
	Metadata      map[string]any      `json:"metadata"`
	Targets       []ManageTargetInput `json:"targets"`
}

type ManageTargetInput struct {
	ChannelAccountID int  `json:"channel_account_id"`
	Enabled          bool `json:"enabled"`
	Priority         int  `json:"priority"`
	Weight           int  `json:"weight"`
}

type RuleView struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	Enabled       bool           `json:"enabled"`
	Priority      int            `json:"priority"`
	AppScope      string         `json:"app_scope"`
	AppIDs        []string       `json:"app_ids"`
	PaymentMethod string         `json:"payment_method"`
	PayModes      []string       `json:"pay_modes"`
	Currency      string         `json:"currency"`
	MinAmount     int64          `json:"min_amount"`
	MaxAmount     int64          `json:"max_amount"`
	Terminal      string         `json:"terminal"`
	Metadata      map[string]any `json:"metadata"`
	Targets       []TargetView   `json:"targets"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type TargetView struct {
	ID               int       `json:"id"`
	RoutingRuleID    int       `json:"routing_rule_id"`
	ChannelAccountID int       `json:"channel_account_id"`
	Enabled          bool      `json:"enabled"`
	Priority         int       `json:"priority"`
	Weight           int       `json:"weight"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type RouteInput struct {
	AppID         string
	PaymentMethod string
	PayMode       string
	Amount        int64
	Currency      string
	Terminal      string
}

type RouteCandidate struct {
	RuleID           int    `json:"rule_id"`
	TargetID         int    `json:"target_id"`
	ChannelAccountID int    `json:"channel_account_id"`
	Channel          string `json:"channel"`
	PaymentMethod    string `json:"payment_method"`
	PayMode          string `json:"pay_mode"`
	Priority         int    `json:"priority"`
	TargetPriority   int    `json:"target_priority"`
}

func (s Service) ListRules(ctx context.Context) ([]RuleView, error) {
	aggregates, err := s.rules.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]RuleView, 0, len(aggregates))
	for _, aggregate := range aggregates {
		items = append(items, toView(aggregate))
	}
	return items, nil
}

func (s Service) GetRule(ctx context.Context, id int) (*RuleView, error) {
	aggregate, err := s.rules.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view := toView(*aggregate)
	return &view, nil
}

func (s Service) CreateRule(ctx context.Context, input ManageRuleInput) (*RuleView, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, err
	}
	created, err := s.rules.Create(ctx, toMutation(normalized))
	if err != nil {
		return nil, err
	}
	view := toView(*created)
	return &view, nil
}

func (s Service) UpdateRule(ctx context.Context, id int, input ManageRuleInput) (*RuleView, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, err
	}
	updated, err := s.rules.Update(ctx, id, toMutation(normalized))
	if err != nil {
		return nil, err
	}
	view := toView(*updated)
	return &view, nil
}

func (s Service) SetRuleEnabled(ctx context.Context, id int, enabled bool) (*RuleView, error) {
	updated, err := s.rules.SetEnabled(ctx, id, enabled)
	if err != nil {
		return nil, err
	}
	view := toView(*updated)
	return &view, nil
}

func (s Service) Resolve(ctx context.Context, input RouteInput) ([]RouteCandidate, error) {
	aggregates, err := s.rules.List(ctx)
	if err != nil {
		return nil, err
	}
	normalized := RouteInput{
		AppID:         strings.TrimSpace(input.AppID),
		PaymentMethod: strings.ToLower(strings.TrimSpace(input.PaymentMethod)),
		PayMode:       strings.ToLower(strings.TrimSpace(input.PayMode)),
		Amount:        input.Amount,
		Currency:      strings.ToUpper(strings.TrimSpace(input.Currency)),
		Terminal:      normalizeTerminal(input.Terminal),
	}
	candidates := make([]RouteCandidate, 0, len(aggregates))
	for _, aggregate := range aggregates {
		if !matchesRule(aggregate.Rule, normalized) {
			continue
		}
		for _, target := range aggregate.Targets {
			if target == nil || !target.Enabled {
				continue
			}
			account, err := s.channels.FindEnabledByID(ctx, target.ChannelAccountID)
			if err != nil {
				if ent.IsNotFound(err) {
					continue
				}
				return nil, err
			}
			if !strings.EqualFold(account.Channel, aggregate.Rule.PaymentMethod) {
				continue
			}
			candidates = append(candidates, RouteCandidate{
				RuleID:           aggregate.Rule.ID,
				TargetID:         target.ID,
				ChannelAccountID: account.ID,
				Channel:          account.Channel,
				PaymentMethod:    aggregate.Rule.PaymentMethod,
				PayMode:          normalized.PayMode,
				Priority:         aggregate.Rule.Priority,
				TargetPriority:   target.Priority,
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			if candidates[i].TargetPriority == candidates[j].TargetPriority {
				return candidates[i].TargetID < candidates[j].TargetID
			}
			return candidates[i].TargetPriority > candidates[j].TargetPriority
		}
		return candidates[i].Priority > candidates[j].Priority
	})
	return candidates, nil
}

func normalizeInput(input ManageRuleInput) (ManageRuleInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ManageRuleInput{}, ErrRuleNameRequired
	}
	appScope := strings.ToLower(strings.TrimSpace(input.AppScope))
	if appScope == "" {
		appScope = AppScopeAll
	}
	if appScope != AppScopeAll && appScope != AppScopeInclude {
		return ManageRuleInput{}, ErrInvalidAppScope
	}
	paymentMethod := strings.ToLower(strings.TrimSpace(input.PaymentMethod))
	if !isValidPaymentMethod(paymentMethod) {
		return ManageRuleInput{}, ErrInvalidPaymentMethod
	}
	terminal := normalizeTerminal(input.Terminal)
	if terminal != "any" && terminal != "desktop" && terminal != "mobile" && terminal != "wechat_browser" {
		return ManageRuleInput{}, ErrInvalidTerminal
	}
	if input.MinAmount > 0 && input.MaxAmount > 0 && input.MinAmount > input.MaxAmount {
		return ManageRuleInput{}, ErrInvalidAmountRange
	}
	if len(input.Targets) == 0 {
		return ManageRuleInput{}, ErrRoutingTargetRequired
	}
	targets := make([]ManageTargetInput, 0, len(input.Targets))
	for _, target := range input.Targets {
		if target.ChannelAccountID <= 0 {
			return ManageRuleInput{}, ErrInvalidTargetChannelAccountID
		}
		priority := target.Priority
		if priority == 0 {
			priority = 100
		}
		weight := target.Weight
		if weight == 0 {
			weight = 100
		}
		targets = append(targets, ManageTargetInput{
			ChannelAccountID: target.ChannelAccountID,
			Enabled:          target.Enabled,
			Priority:         priority,
			Weight:           weight,
		})
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	priority := input.Priority
	if priority == 0 {
		priority = 100
	}
	return ManageRuleInput{
		Name:          name,
		Enabled:       input.Enabled,
		Priority:      priority,
		AppScope:      appScope,
		AppIDs:        normalizeStringList(input.AppIDs, false),
		PaymentMethod: paymentMethod,
		PayModes:      normalizeStringList(input.PayModes, false),
		Currency:      strings.ToUpper(strings.TrimSpace(input.Currency)),
		MinAmount:     input.MinAmount,
		MaxAmount:     input.MaxAmount,
		Terminal:      terminal,
		Metadata:      metadata,
		Targets:       targets,
	}, nil
}

func normalizeStringList(values []string, uppercase bool) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if uppercase {
			item = strings.ToUpper(item)
		} else {
			item = strings.ToLower(item)
		}
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}

func normalizeTerminal(value string) string {
	terminal := strings.ToLower(strings.TrimSpace(value))
	if terminal == "" {
		return "any"
	}
	return terminal
}

func isValidPaymentMethod(value string) bool {
	return value == "wechat" || value == "alipay" || value == "paypal"
}

func matchesRule(rule *ent.RoutingRule, input RouteInput) bool {
	if rule == nil || !rule.Enabled {
		return false
	}
	if rule.AppScope == AppScopeInclude && !contains(rule.AppIds, input.AppID) {
		return false
	}
	if rule.PaymentMethod != input.PaymentMethod {
		return false
	}
	if len(rule.PayModes) > 0 && !contains(rule.PayModes, input.PayMode) {
		return false
	}
	if strings.TrimSpace(rule.Currency) != "" && strings.ToUpper(strings.TrimSpace(rule.Currency)) != input.Currency {
		return false
	}
	if rule.MinAmount > 0 && input.Amount < rule.MinAmount {
		return false
	}
	if rule.MaxAmount > 0 && input.Amount > rule.MaxAmount {
		return false
	}
	terminal := normalizeTerminal(rule.Terminal)
	if terminal != "any" && terminal != input.Terminal {
		return false
	}
	return paymentsvc.ChannelSupportsCurrency(rule.PaymentMethod, input.Currency)
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func toMutation(input ManageRuleInput) routingrepo.RuleMutation {
	targets := make([]routingrepo.TargetMutation, 0, len(input.Targets))
	for _, target := range input.Targets {
		targets = append(targets, routingrepo.TargetMutation{
			ChannelAccountID: target.ChannelAccountID,
			Enabled:          target.Enabled,
			Priority:         target.Priority,
			Weight:           target.Weight,
		})
	}
	return routingrepo.RuleMutation{
		Name:          input.Name,
		Enabled:       input.Enabled,
		Priority:      input.Priority,
		AppScope:      input.AppScope,
		AppIDs:        input.AppIDs,
		PaymentMethod: input.PaymentMethod,
		PayModes:      input.PayModes,
		Currency:      input.Currency,
		MinAmount:     input.MinAmount,
		MaxAmount:     input.MaxAmount,
		Terminal:      input.Terminal,
		Metadata:      input.Metadata,
		Targets:       targets,
	}
}

func toView(aggregate routingrepo.RuleAggregate) RuleView {
	targets := make([]TargetView, 0, len(aggregate.Targets))
	for _, target := range aggregate.Targets {
		targets = append(targets, TargetView{
			ID:               target.ID,
			RoutingRuleID:    target.RoutingRuleID,
			ChannelAccountID: target.ChannelAccountID,
			Enabled:          target.Enabled,
			Priority:         target.Priority,
			Weight:           target.Weight,
			CreatedAt:        target.CreatedAt,
			UpdatedAt:        target.UpdatedAt,
		})
	}
	rule := aggregate.Rule
	return RuleView{
		ID:            rule.ID,
		Name:          rule.Name,
		Enabled:       rule.Enabled,
		Priority:      rule.Priority,
		AppScope:      rule.AppScope,
		AppIDs:        rule.AppIds,
		PaymentMethod: rule.PaymentMethod,
		PayModes:      rule.PayModes,
		Currency:      rule.Currency,
		MinAmount:     rule.MinAmount,
		MaxAmount:     rule.MaxAmount,
		Terminal:      rule.Terminal,
		Metadata:      rule.Metadata,
		Targets:       targets,
		CreatedAt:     rule.CreatedAt,
		UpdatedAt:     rule.UpdatedAt,
	}
}
