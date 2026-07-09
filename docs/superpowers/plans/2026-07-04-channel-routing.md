# Channel Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a real channel routing layer so checkout chooses payment channels through configurable rules instead of hard-coded enabled-channel filtering.

**Architecture:** Add a vertical `internal/domain/routing` module with Ent-backed rules, a route decision service, admin CRUD APIs, and frontend management pages. Checkout will call routing service to get eligible methods, while payment start still validates channel availability, currency, and locked order constraints.

**Tech Stack:** Go 1.26, Gin, Ent, PostgreSQL, Redis, existing orders/payments/channels services, React 19, Rsbuild, Bun, shadcn/ui, reusable Data Table.

## Global Constraints

- Backend responses must use `{ code, message, data, error }` via `internal/platform/httpx`.
- Tests for routing must live under `internal/domain/routing/test/`.
- Money remains integer minor units.
- `channel` and `pay_method` remain the same value for `wechat`, `alipay`, and `paypal` in this phase.
- `alipay` and `wechat` support `CNY`; `paypal` supports configured PayPal currencies.
- Checkout must still show a locked direct-pay UI when the order already has `channel/pay_method`.
- If routing finds no available channel, checkout returns an empty method list and the frontend shows “no available methods”.
- Frontend UI must use shadcn/ui and shared Data Table components.
- After backend code changes, build `.tmp/payment-gateway-server` and restart backend from that binary.

---

## File Structure

- Create `ent/schema/routing_rule.go`: persistent route rules.
- Generate Ent code with `make ent-up`.
- Create `internal/domain/routing/repository/repository.go`: Ent CRUD and list queries.
- Create `internal/domain/routing/service/service.go`: validation, CRUD, and route decision.
- Create `internal/domain/routing/handler/handler.go`: admin APIs and route preview endpoint.
- Create `internal/domain/routing/router/router.go`: route registration.
- Create `internal/domain/routing/test/service_test.go`: rule matching tests.
- Create `internal/domain/routing/test/handler_test.go`: API behavior tests.
- Modify `internal/domain/orders/handler/checkout_handler.go`: use routing service in `ListPaymentMethods`.
- Modify `internal/platform/http/router.go`: wire routing service into admin and checkout.
- Modify `internal/platform/rbac/defaults.go`: add routing permissions if RBAC policy file requires it.
- Create `web/src/features/routing/types.ts`, `api.ts`, `routing-page.tsx`, `routing-form-page.tsx`.
- Create `web/src/routes/routing.tsx`.
- Modify `web/src/router.tsx`, `web/src/routes/root.tsx`, `web/src/i18n/resources.ts`.

## Backend API Contract

Admin endpoints:

- `GET /v1/admin/routing-rules`
- `POST /v1/admin/routing-rules`
- `GET /v1/admin/routing-rules/:id`
- `PUT /v1/admin/routing-rules/:id`
- `POST /v1/admin/routing-rules/:id/enable`
- `POST /v1/admin/routing-rules/:id/disable`
- `POST /v1/admin/routing-rules/preview`

Rule request:

```json
{
  "name": "CNY 默认支付宝",
  "enabled": true,
  "priority": 100,
  "app_id": "",
  "currency": "CNY",
  "min_amount": 0,
  "max_amount": 0,
  "terminal": "any",
  "channel": "alipay",
  "pay_method": "alipay",
  "pay_mode": "",
  "metadata": {}
}
```

Field rules:

- Empty `app_id` means all apps.
- Empty `currency` means all currencies supported by the selected channel.
- `min_amount = 0` means no lower bound.
- `max_amount = 0` means no upper bound.
- `terminal`: `any`, `desktop`, `mobile`.
- `priority`: higher number wins.
- If multiple rules have the same priority, use lower `id` first for stable matching.

---

### Task 1: Add RoutingRule Schema and Generated Ent Code

**Files:**
- Create: `ent/schema/routing_rule.go`
- Generated: `ent/*`

**Interfaces:**
- Produces Ent type `ent.RoutingRule`.
- Fields: `name`, `enabled`, `priority`, `app_id`, `currency`, `min_amount`, `max_amount`, `terminal`, `channel`, `pay_method`, `pay_mode`, `metadata`, `created_at`, `updated_at`.

- [ ] **Step 1: Write the schema**

Create `ent/schema/routing_rule.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RoutingRule struct {
	ent.Schema
}

func (RoutingRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(100),
		field.String("app_id").Optional(),
		field.String("currency").Optional(),
		field.Int64("min_amount").Default(0),
		field.Int64("max_amount").Default(0),
		field.String("terminal").Default("any"),
		field.String("channel"),
		field.String("pay_method"),
		field.String("pay_mode").Optional(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RoutingRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("priority"),
		index.Fields("app_id"),
		index.Fields("currency"),
		index.Fields("channel"),
		index.Fields("created_at"),
	}
}
```

- [ ] **Step 2: Generate Ent code**

Run: `make ent-up`

Expected: generated files include `ent/routingrule`, `ent/routingrule.go`, and client methods.

- [ ] **Step 3: Verify compile**

Run: `go test ./ent/...`

Expected: PASS.

### Task 2: Implement Routing Repository and Service CRUD

**Files:**
- Create: `internal/domain/routing/repository/repository.go`
- Create: `internal/domain/routing/service/service.go`
- Test: `internal/domain/routing/test/service_test.go`

**Interfaces:**
- `service.New(client *ent.Client, opts ...Option) Service`
- `Service.ListRules(ctx context.Context) ([]RuleView, error)`
- `Service.GetRule(ctx context.Context, id int) (*RuleView, error)`
- `Service.CreateRule(ctx context.Context, input ManageRuleInput) (*RuleView, error)`
- `Service.UpdateRule(ctx context.Context, id int, input ManageRuleInput) (*RuleView, error)`
- `Service.SetRuleEnabled(ctx context.Context, id int, enabled bool) (*RuleView, error)`

- [ ] **Step 1: Write failing service tests**

Add tests for:

```go
func TestCreateRuleRejectsInvalidChannel(t *testing.T) {}
func TestCreateRuleRejectsInvalidTerminal(t *testing.T) {}
func TestCreateRuleNormalizesCurrencyAndChannel(t *testing.T) {}
func TestSetRuleEnabledTogglesRule(t *testing.T) {}
```

Run: `go test ./internal/domain/routing/test -run TestCreateRule -count=1`

Expected: FAIL because service does not exist.

- [ ] **Step 2: Implement repository**

Repository methods:

```go
func New(client *ent.Client) Repository
func (r Repository) IsZero() bool
func (r Repository) List(ctx context.Context) ([]*ent.RoutingRule, error)
func (r Repository) FindByID(ctx context.Context, id int) (*ent.RoutingRule, error)
func (r Repository) Create(ctx context.Context, input RuleMutation) (*ent.RoutingRule, error)
func (r Repository) Update(ctx context.Context, id int, input RuleMutation) (*ent.RoutingRule, error)
func (r Repository) SetEnabled(ctx context.Context, id int, enabled bool) (*ent.RoutingRule, error)
```

- [ ] **Step 3: Implement service validation**

Validation:

```go
var (
	ErrRuleNameRequired = errors.New("routing rule name is required")
	ErrInvalidChannel = errors.New("invalid routing channel")
	ErrInvalidTerminal = errors.New("invalid routing terminal")
	ErrInvalidAmountRange = errors.New("invalid routing amount range")
)
```

Valid channels: `wechat`, `alipay`, `paypal`.

Valid terminals: `any`, `desktop`, `mobile`.

If `pay_method` is empty, set it to `channel`.

Normalize `channel/pay_method` to lowercase and `currency` to uppercase.

- [ ] **Step 4: Verify service tests**

Run: `go test ./internal/domain/routing/test -count=1`

Expected: PASS.

### Task 3: Implement Route Decision Engine

**Files:**
- Modify: `internal/domain/routing/service/service.go`
- Test: `internal/domain/routing/test/service_test.go`

**Interfaces:**
- `RouteInput{AppID string, Amount int64, Currency string, Terminal string}`
- `RouteCandidate{RuleID int, Channel string, PayMethod string, PayMode string, Priority int}`
- `Service.Resolve(ctx context.Context, input RouteInput) ([]RouteCandidate, error)`

- [ ] **Step 1: Write failing route tests**

Add tests:

```go
func TestResolveReturnsHighestPriorityMatchingRuleFirst(t *testing.T) {}
func TestResolveSkipsDisabledRules(t *testing.T) {}
func TestResolveFiltersByAppCurrencyAmountAndTerminal(t *testing.T) {}
func TestResolveReturnsEmptyWhenNoRulesMatch(t *testing.T) {}
```

Run: `go test ./internal/domain/routing/test -run TestResolve -count=1`

Expected: FAIL because `Resolve` is not implemented.

- [ ] **Step 2: Implement matching logic**

Matching rules:

- rule must be enabled.
- rule `app_id` matches when empty or equal to order app id.
- rule `currency` matches when empty or equal to order currency.
- `min_amount > 0` requires `amount >= min_amount`.
- `max_amount > 0` requires `amount <= max_amount`.
- terminal matches when `any` or equal to detected terminal.
- channel must support the order currency via `paymentsvc.ChannelSupportsCurrency`.

Sort by:

1. `priority DESC`
2. `id ASC`

- [ ] **Step 3: Verify route tests**

Run: `go test ./internal/domain/routing/test -run TestResolve -count=1`

Expected: PASS.

### Task 4: Wire Routing Into Checkout Method Listing

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/platform/http/router.go`
- Test: `internal/domain/orders/test/checkout_payment_test.go`

**Interfaces:**
- `orderhandler.WithRoutingService(routing.Service)` option.
- Checkout calls routing only for unlocked orders.

- [ ] **Step 1: Write failing checkout tests**

Add tests:

```go
func TestCheckoutHandlerUsesRoutingRulesForUnlockedOrder(t *testing.T) {}
func TestCheckoutHandlerFallsBackToEnabledChannelsWhenNoRoutingRulesExist(t *testing.T) {}
func TestCheckoutHandlerKeepsLockedOrderIndependentFromRoutingRules(t *testing.T) {}
func TestCheckoutHandlerSkipsRoutedChannelWhenAccountDisabled(t *testing.T) {}
```

Expected behavior:

- If a matching rule routes CNY to `wechat`, methods show `wechat` first.
- If no rules exist, current enabled-channel behavior remains.
- If order is locked to `alipay`, routing does not override it.
- If a routed channel account is disabled or missing, it is not shown.

- [ ] **Step 2: Add `WithRoutingService` option**

Extend `CheckoutHandler`:

```go
type CheckoutHandler struct {
	routing routing.Service
}
```

Add:

```go
func WithRoutingService(routingService routing.Service) CheckoutOption {
	return func(h *CheckoutHandler) {
		h.routing = routingService
	}
}
```

- [ ] **Step 3: Replace unlocked method building**

For unlocked orders:

1. Detect terminal from UA.
2. Call `routing.Resolve`.
3. Convert route candidates to checkout methods.
4. Validate account exists and is enabled.
5. Apply channel-specific pay mode resolution:
   - Alipay keeps existing `selectAlipayPayMode`.
   - WeChat uses candidate `pay_mode` if present; otherwise channel config `mode`.
6. If no route candidates exist, fall back to current enabled-channel listing.

- [ ] **Step 4: Verify checkout tests**

Run: `go test ./internal/domain/orders/test -run TestCheckoutHandler.*Routing -count=1`

Expected: PASS.

### Task 5: Add Admin Routing APIs and RBAC

**Files:**
- Create: `internal/domain/routing/handler/handler.go`
- Create: `internal/domain/routing/router/router.go`
- Create: `internal/domain/routing/test/handler_test.go`
- Modify: `internal/platform/http/router.go`
- Modify: `internal/platform/rbac/defaults.go`

**Interfaces:**
- Admin endpoints under `/v1/admin/routing-rules`.

- [ ] **Step 1: Write failing handler tests**

Add tests:

```go
func TestRoutingHandlerCreatesRule(t *testing.T) {}
func TestRoutingHandlerListsRules(t *testing.T) {}
func TestRoutingHandlerPreviewsRoute(t *testing.T) {}
```

Run: `go test ./internal/domain/routing/test -run TestRoutingHandler -count=1`

Expected: FAIL because routes do not exist.

- [ ] **Step 2: Implement handler**

Handlers:

- `ListRules`
- `GetRule`
- `CreateRule`
- `UpdateRule`
- `EnableRule`
- `DisableRule`
- `Preview`

Use `httpx.JSONOK` and `httpx.JSONError`.

- [ ] **Step 3: Register router**

Register:

```go
routingService := routingsvc.New(client)
routingrouter.Register(router.Group("/v1/admin"), routinghandler.New(routingService), userService, enforcer)
```

Pass `routingService` into checkout with `orderhandler.WithRoutingService(routingService)`.

- [ ] **Step 4: Verify handler tests**

Run: `go test ./internal/domain/routing/test -count=1`

Expected: PASS.

### Task 6: Build Routing Frontend

**Files:**
- Create: `web/src/features/routing/types.ts`
- Create: `web/src/features/routing/api.ts`
- Create: `web/src/features/routing/routing-page.tsx`
- Create: `web/src/features/routing/routing-form-page.tsx`
- Create: `web/src/routes/routing.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Uses admin API `/v1/admin/routing-rules`.
- Uses shared Data Table factory for list display.

- [ ] **Step 1: Add TypeScript types and API client**

Define:

```ts
export type RoutingRule = {
  id: number;
  name: string;
  enabled: boolean;
  priority: number;
  app_id?: string;
  currency?: string;
  min_amount: number;
  max_amount: number;
  terminal: "any" | "desktop" | "mobile";
  channel: "wechat" | "alipay" | "paypal";
  pay_method: string;
  pay_mode?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};
```

- [ ] **Step 2: Build list page**

Use shadcn Button, Badge, Switch, Select, and shared Data Table.

Columns:

- name
- enabled
- priority
- app_id
- currency
- amount range
- terminal
- channel
- pay_mode
- actions

- [ ] **Step 3: Build create/edit form page**

Use shadcn Card, Input, Select, Switch, Textarea.

Do not use a modal for create/edit.

- [ ] **Step 4: Add sidebar route and translations**

Add `/routing` to router and sidebar under payment operations.

Add Chinese and English translation keys.

- [ ] **Step 5: Verify frontend**

Run:

```bash
make web-typecheck
make web-build
```

Expected: PASS.

### Task 7: Full Verification and Backend Restart

**Files:**
- No new source files unless previous tasks reveal compile errors.

- [ ] **Step 1: Run backend tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Build latest backend**

Run:

```bash
go build -o .tmp/payment-gateway-server ./cmd/server
```

Expected: binary builds successfully.

- [ ] **Step 3: Restart backend**

Stop any old backend:

```bash
ps -ef | rg 'payment-gateway-server|cmd/server' | rg -v rg
kill <pid>
```

Start latest binary:

```bash
set -a && . ./.env && set +a && ./.tmp/payment-gateway-server
```

- [ ] **Step 4: Smoke test**

Run:

```bash
curl -sS http://127.0.0.1:8080/healthz
```

Expected:

```json
{"code":"ok","data":{"status":"ok"},"error":null,"message":"ok"}
```

## Self-Review

- Spec coverage: plan covers persistent rules, route decision, checkout integration, admin API, frontend UI, tests, and backend restart.
- Placeholder scan: no `TBD`, `TODO`, or unspecified implementation steps remain.
- Type consistency: route fields match Ent schema, backend service types, frontend types, and API contract.
- Scope control: first phase does not implement cost-based routing, weighted random distribution, channel health scoring, or automatic failover after provider errors. Those should be a second-phase routing optimization after rule routing is stable.
