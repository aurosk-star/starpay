package routingtest

import (
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	channelsvc "payment-gateway/internal/domain/channels/service"
	routingsvc "payment-gateway/internal/domain/routing/service"
)

func TestCreateRuleRejectsInvalidPaymentMethod(t *testing.T) {
	client, svc := newService(t, "routing_invalid_payment_method")
	accountID := mustCreateChannelAccount(t, client, "alipay", true)

	_, err := svc.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "invalid",
		PaymentMethod: "stripe",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: accountID,
			Enabled:          true,
		}},
	})
	if !errors.Is(err, routingsvc.ErrInvalidPaymentMethod) {
		t.Fatalf("CreateRule() error = %v, want ErrInvalidPaymentMethod", err)
	}
}

func TestCreateRuleRejectsInvalidAppScope(t *testing.T) {
	client, svc := newService(t, "routing_invalid_app_scope")
	accountID := mustCreateChannelAccount(t, client, "alipay", true)

	_, err := svc.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "invalid",
		AppScope:      "exclude",
		PaymentMethod: "alipay",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: accountID,
			Enabled:          true,
		}},
	})
	if !errors.Is(err, routingsvc.ErrInvalidAppScope) {
		t.Fatalf("CreateRule() error = %v, want ErrInvalidAppScope", err)
	}
}

func TestCreateRuleRejectsEmptyTargets(t *testing.T) {
	_, svc := newService(t, "routing_empty_targets")

	_, err := svc.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "empty targets",
		PaymentMethod: "alipay",
	})
	if !errors.Is(err, routingsvc.ErrRoutingTargetRequired) {
		t.Fatalf("CreateRule() error = %v, want ErrRoutingTargetRequired", err)
	}
}

func TestCreateRuleRejectsInvalidTargetChannelAccountID(t *testing.T) {
	_, svc := newService(t, "routing_invalid_target_account")

	_, err := svc.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "invalid target",
		PaymentMethod: "alipay",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: 0,
			Enabled:          true,
		}},
	})
	if !errors.Is(err, routingsvc.ErrInvalidTargetChannelAccountID) {
		t.Fatalf("CreateRule() error = %v, want ErrInvalidTargetChannelAccountID", err)
	}
}

func TestCreateRuleAcceptsMultipleAppsModesAndTargets(t *testing.T) {
	client, svc := newService(t, "routing_multi_matchers")
	firstAccountID := mustCreateChannelAccount(t, client, "wechat", true)
	secondAccountID := mustCreateChannelAccount(t, client, "wechat", true)

	rule, err := svc.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          " 微信 CNY 默认路由 ",
		Enabled:       true,
		Priority:      0,
		AppScope:      "include",
		AppIDs:        []string{" snsgo ", "shop", "snsgo"},
		PaymentMethod: " WeChat ",
		PayModes:      []string{" NATIVE ", "h5", "native"},
		Currency:      " cny ",
		Terminal:      " DESKTOP ",
		Targets: []routingsvc.ManageTargetInput{
			{ChannelAccountID: firstAccountID, Enabled: true, Priority: 0, Weight: 0},
			{ChannelAccountID: secondAccountID, Enabled: true, Priority: 80, Weight: 50},
		},
	})
	if err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}
	if rule.Name != "微信 CNY 默认路由" || rule.Priority != 100 || rule.PaymentMethod != "wechat" || rule.Currency != "CNY" || rule.Terminal != "desktop" {
		t.Fatalf("rule = %#v, want normalized rule fields", rule)
	}
	if got, want := rule.AppIDs, []string{"snsgo", "shop"}; !stringSlicesEqual(got, want) {
		t.Fatalf("AppIDs = %#v, want %#v", got, want)
	}
	if got, want := rule.PayModes, []string{"native", "h5"}; !stringSlicesEqual(got, want) {
		t.Fatalf("PayModes = %#v, want %#v", got, want)
	}
	if len(rule.Targets) != 2 {
		t.Fatalf("targets len = %d, want 2", len(rule.Targets))
	}
	if rule.Targets[0].Priority != 100 || rule.Targets[0].Weight != 100 {
		t.Fatalf("first target = %#v, want default priority and weight", rule.Targets[0])
	}
}

func TestSetRuleEnabledTogglesRule(t *testing.T) {
	client, svc := newService(t, "routing_toggle")
	rule := mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "paypal",
		Enabled:       true,
		PaymentMethod: "paypal",
		Terminal:      "any",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: mustCreateChannelAccount(t, client, "paypal", true),
			Enabled:          true,
		}},
	})

	disabled, err := svc.SetRuleEnabled(t.Context(), rule.ID, false)
	if err != nil {
		t.Fatalf("SetRuleEnabled(false) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("Enabled = true, want false")
	}
}

func TestResolveMatchesAppPaymentMethodPayModeAndTargets(t *testing.T) {
	client, svc := newService(t, "routing_resolve_matchers")
	wechatAccountID := mustCreateChannelAccount(t, client, "wechat", true)
	rule := mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "wechat CNY",
		Enabled:       true,
		Priority:      100,
		AppScope:      "include",
		AppIDs:        []string{"snsgo", "shop"},
		PaymentMethod: "wechat",
		PayModes:      []string{"native", "h5"},
		Currency:      "CNY",
		MinAmount:     1000,
		MaxAmount:     20000,
		Terminal:      "desktop",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: wechatAccountID,
			Enabled:          true,
			Priority:         100,
		}},
	})

	candidates, err := svc.Resolve(t.Context(), routingsvc.RouteInput{
		AppID:         "snsgo",
		PaymentMethod: "wechat",
		PayMode:       "native",
		Amount:        9900,
		Currency:      "cny",
		Terminal:      "desktop",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1", len(candidates))
	}
	if candidates[0].RuleID != rule.ID || candidates[0].ChannelAccountID != wechatAccountID || candidates[0].PaymentMethod != "wechat" || candidates[0].PayMode != "native" {
		t.Fatalf("candidate = %#v, want matched route target", candidates[0])
	}
}

func TestResolveSkipsDisabledTargetAndDisabledChannelAccount(t *testing.T) {
	client, svc := newService(t, "routing_resolve_disabled_target")
	disabledTargetAccountID := mustCreateChannelAccount(t, client, "alipay", true)
	disabledAccountID := mustCreateChannelAccount(t, client, "alipay", false)
	enabledAccountID := mustCreateChannelAccount(t, client, "alipay", true)
	mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "alipay CNY",
		Enabled:       true,
		Priority:      100,
		PaymentMethod: "alipay",
		PayModes:      []string{"page"},
		Currency:      "CNY",
		Terminal:      "any",
		Targets: []routingsvc.ManageTargetInput{
			{ChannelAccountID: disabledTargetAccountID, Enabled: false, Priority: 300},
			{ChannelAccountID: disabledAccountID, Enabled: true, Priority: 200},
			{ChannelAccountID: enabledAccountID, Enabled: true, Priority: 100},
		},
	})

	candidates, err := svc.Resolve(t.Context(), routingsvc.RouteInput{
		PaymentMethod: "alipay",
		PayMode:       "page",
		Amount:        9900,
		Currency:      "CNY",
		Terminal:      "desktop",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ChannelAccountID != enabledAccountID {
		t.Fatalf("candidates = %#v, want only enabled target with enabled account", candidates)
	}
}

func TestResolveSkipsTargetWithMismatchedChannelAccount(t *testing.T) {
	client, svc := newService(t, "routing_resolve_mismatched_target")
	wechatAccountID := mustCreateChannelAccount(t, client, "wechat", true)
	alipayAccountID := mustCreateChannelAccount(t, client, "alipay", true)
	mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "alipay CNY",
		Enabled:       true,
		Priority:      100,
		PaymentMethod: "alipay",
		Currency:      "CNY",
		Terminal:      "any",
		Targets: []routingsvc.ManageTargetInput{
			{ChannelAccountID: wechatAccountID, Enabled: true, Priority: 200},
			{ChannelAccountID: alipayAccountID, Enabled: true, Priority: 100},
		},
	})

	candidates, err := svc.Resolve(t.Context(), routingsvc.RouteInput{
		PaymentMethod: "alipay",
		Amount:        9900,
		Currency:      "CNY",
		Terminal:      "desktop",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ChannelAccountID != alipayAccountID {
		t.Fatalf("candidates = %#v, want only alipay channel account", candidates)
	}
}

func TestResolveSortsByRulePriorityThenTargetPriority(t *testing.T) {
	client, svc := newService(t, "routing_resolve_priority")
	lowRuleAccountID := mustCreateChannelAccount(t, client, "paypal", true)
	highRuleLowTargetAccountID := mustCreateChannelAccount(t, client, "paypal", true)
	highRuleHighTargetAccountID := mustCreateChannelAccount(t, client, "paypal", true)
	low := mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "low rule",
		Enabled:       true,
		Priority:      10,
		PaymentMethod: "paypal",
		PayModes:      []string{"checkout"},
		Currency:      "USD",
		Terminal:      "any",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: lowRuleAccountID,
			Enabled:          true,
			Priority:         300,
		}},
	})
	high := mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "high rule",
		Enabled:       true,
		Priority:      90,
		PaymentMethod: "paypal",
		PayModes:      []string{"checkout"},
		Currency:      "USD",
		Terminal:      "any",
		Targets: []routingsvc.ManageTargetInput{
			{ChannelAccountID: highRuleLowTargetAccountID, Enabled: true, Priority: 100},
			{ChannelAccountID: highRuleHighTargetAccountID, Enabled: true, Priority: 200},
		},
	})

	candidates, err := svc.Resolve(t.Context(), routingsvc.RouteInput{
		PaymentMethod: "paypal",
		PayMode:       "checkout",
		Amount:        9900,
		Currency:      "USD",
		Terminal:      "desktop",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidates len = %d, want 3", len(candidates))
	}
	if candidates[0].RuleID != high.ID || candidates[0].ChannelAccountID != highRuleHighTargetAccountID {
		t.Fatalf("first candidate = %#v, want high rule high target", candidates[0])
	}
	if candidates[1].RuleID != high.ID || candidates[1].ChannelAccountID != highRuleLowTargetAccountID {
		t.Fatalf("second candidate = %#v, want high rule low target", candidates[1])
	}
	if candidates[2].RuleID != low.ID || candidates[2].ChannelAccountID != lowRuleAccountID {
		t.Fatalf("third candidate = %#v, want low rule target", candidates[2])
	}
}

func TestResolveReturnsEmptyWhenNoRulesMatch(t *testing.T) {
	client, svc := newService(t, "routing_resolve_empty")
	mustCreateRule(t, svc, routingsvc.ManageRuleInput{
		Name:          "cny",
		Enabled:       true,
		Currency:      "CNY",
		Terminal:      "desktop",
		PaymentMethod: "alipay",
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: mustCreateChannelAccount(t, client, "alipay", true),
			Enabled:          true,
		}},
	})

	candidates, err := svc.Resolve(t.Context(), routingsvc.RouteInput{
		PaymentMethod: "alipay",
		Amount:        9900,
		Currency:      "USD",
		Terminal:      "desktop",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates len = %d, want 0", len(candidates))
	}
}

func newService(t *testing.T, dbName string) (*ent.Client, routingsvc.Service) {
	t.Helper()
	client := enttest.Open(t, dialect.SQLite, "file:"+dbName+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	return client, routingsvc.New(client)
}

func mustCreateChannelAccount(t *testing.T, client *ent.Client, channel string, enabled bool) int {
	t.Helper()
	account, err := channelsvc.New(client).CreateChannelAccount(t.Context(), channelsvc.ManageChannelAccountInput{
		Channel: channel,
		Name:    channel + " account",
		Enabled: enabled,
		Env:     "sandbox",
		Config:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount(%s) error = %v", channel, err)
	}
	return account.ID
}

func mustCreateRule(t *testing.T, svc routingsvc.Service, input routingsvc.ManageRuleInput) *routingsvc.RuleView {
	t.Helper()
	rule, err := svc.CreateRule(t.Context(), input)
	if err != nil {
		t.Fatalf("CreateRule(%s) error = %v", input.Name, err)
	}
	return rule
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
