# Channel Routing Targets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor channel routing from “choose a channel type” into “match rules, then choose one or more concrete channel accounts.”

**Architecture:** Keep `routing` as a vertical domain, but split route conditions from route targets. `RoutingRule` stores matching scope: apps, payment method, pay modes, currency, amount, terminal, priority. `RoutingTarget` stores candidate channel accounts under a rule. Checkout resolves `payment_method + pay_mode`, asks routing for candidates, then starts payment with the selected channel account.

**Tech Stack:** Go 1.26, Gin, Ent, PostgreSQL, Redis, existing `channels/orders/payments` services, React 19, Rsbuild, Bun, shadcn/ui, reusable Data Table.

## Global Constraints

- Backend responses must use `{ code, message, data, error }` via `internal/platform/httpx`.
- Tests for routing must live under `internal/domain/routing/test/`.
- Money remains integer minor units.
- One routing rule can match multiple apps.
- One routing rule can match multiple pay modes.
- One routing rule can have multiple channel-account targets.
- A target channel account must be enabled and must belong to the matched payment method.
- `payment_method` means user-facing method: `wechat`, `alipay`, `paypal`.
- `pay_mode` means method-specific launch mode: WeChat `native/h5/jsapi`, Alipay `page/wap/qr`, PayPal `checkout`.
- Frontend UI must use shadcn/ui and shared Data Table components.
- After backend code changes, build `.tmp/payment-gateway-server` and restart backend from that binary.

---

## File Structure

- Modify `ent/schema/routing_rule.go`: replace single `app_id/channel/pay_method/pay_mode` target fields with matcher fields.
- Create `ent/schema/routing_target.go`: targets linked to rules and channel accounts by IDs.
- Generate Ent code with `make ent-up`.
- Modify `internal/domain/routing/repository/repository.go`: CRUD rules plus nested targets.
- Modify `internal/domain/routing/service/service.go`: new model, validation, target selection.
- Modify `internal/domain/routing/handler/handler.go`: request/response payloads for rules with targets.
- Modify `internal/domain/orders/handler/checkout_handler.go`: route by payment method/pay mode and pass selected account.
- Modify `internal/domain/payments/service/service.go`: allow `StartPaymentInput` to use selected `ChannelAccountID`.
- Modify `internal/domain/channels/repository/repository.go`: add `FindEnabledByID`.
- Modify `web/src/features/routing/*`: form and table support multi-app, multi-mode, multi-targets.
- Modify `web/src/features/channels/api.ts`: reuse channel account list for target selection.
- Modify `web/src/i18n/resources.ts`: update routing copy.

## Target Business Model

```text
Payment Order
  -> payment_method
  -> pay_mode
  -> RoutingRule match
  -> RoutingTarget select
  -> ChannelAccount
  -> Provider
```

```text
RoutingRule
  app_scope: all/include
  app_ids: ["app_a", "app_b"]
  payment_method: wechat
  pay_modes: ["native", "h5"]
  currency: CNY
  min_amount: 0
  max_amount: 50000
  terminal: any
  priority: 100
  targets:
    - channel_account_id: 1, priority: 100, weight: 100
    - channel_account_id: 2, priority: 90, weight: 100
```

## API Contract

`POST /v1/admin/routing-rules`

```json
{
  "name": "微信 CNY 默认路由",
  "enabled": true,
  "priority": 100,
  "app_scope": "include",
  "app_ids": ["snsgo", "shop"],
  "payment_method": "wechat",
  "pay_modes": ["native", "h5"],
  "currency": "CNY",
  "min_amount": 0,
  "max_amount": 50000,
  "terminal": "any",
  "targets": [
    {
      "channel_account_id": 1,
      "enabled": true,
      "priority": 100,
      "weight": 100
    }
  ]
}
```

Route preview request:

```json
{
  "app_id": "snsgo",
  "payment_method": "wechat",
  "pay_mode": "native",
  "amount": 9900,
  "currency": "CNY",
  "terminal": "desktop"
}
```

Route preview response:

```json
{
  "candidates": [
    {
      "rule_id": 1,
      "target_id": 1,
      "channel_account_id": 1,
      "channel": "wechat",
      "payment_method": "wechat",
      "pay_mode": "native",
      "priority": 100,
      "target_priority": 100
    }
  ]
}
```

---

### Task 1: Refactor Ent Schemas

**Files:**
- Modify: `ent/schema/routing_rule.go`
- Create: `ent/schema/routing_target.go`
- Generated: `ent/*`

**Interfaces:**
- `RoutingRule` fields: `name`, `enabled`, `priority`, `app_scope`, `app_ids`, `payment_method`, `pay_modes`, `currency`, `min_amount`, `max_amount`, `terminal`, `metadata`, timestamps.
- `RoutingTarget` fields: `routing_rule_id`, `channel_account_id`, `enabled`, `priority`, `weight`, timestamps.

- [ ] **Step 1: Update rule schema**

Replace current `RoutingRule` fields with:

```go
field.String("name"),
field.Bool("enabled").Default(true),
field.Int("priority").Default(100),
field.String("app_scope").Default("all"),
field.JSON("app_ids", []string{}).Optional(),
field.String("payment_method"),
field.JSON("pay_modes", []string{}).Optional(),
field.String("currency").Optional(),
field.Int64("min_amount").Default(0),
field.Int64("max_amount").Default(0),
field.String("terminal").Default("any"),
field.JSON("metadata", map[string]any{}).Optional(),
field.Time("created_at").Default(time.Now).Immutable(),
field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
```

Indexes:

```go
index.Fields("enabled"),
index.Fields("priority"),
index.Fields("payment_method"),
index.Fields("currency"),
index.Fields("terminal"),
index.Fields("created_at"),
```

- [ ] **Step 2: Add target schema**

Create `ent/schema/routing_target.go` with:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RoutingTarget struct {
	ent.Schema
}

func (RoutingTarget) Fields() []ent.Field {
	return []ent.Field{
		field.Int("routing_rule_id"),
		field.Int("channel_account_id"),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(100),
		field.Int("weight").Default(100),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RoutingTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("routing_rule_id"),
		index.Fields("channel_account_id"),
		index.Fields("enabled"),
		index.Fields("priority"),
	}
}
```

- [ ] **Step 3: Generate Ent**

Run:

```bash
make ent-up
go test ./ent/...
```

Expected: generated `ent/routingtarget` package exists and tests pass.

### Task 2: Update Routing Service Model and Validation

**Files:**
- Modify: `internal/domain/routing/service/service.go`
- Modify: `internal/domain/routing/repository/repository.go`
- Test: `internal/domain/routing/test/service_test.go`

**Interfaces:**
- `ManageRuleInput` includes `AppScope string`, `AppIDs []string`, `PaymentMethod string`, `PayModes []string`, `Targets []ManageTargetInput`.
- `RuleView` includes nested `Targets []TargetView`.
- `RouteInput` includes `PaymentMethod string`, `PayMode string`.
- `RouteCandidate` includes `TargetID`, `ChannelAccountID`, `Channel`, `PaymentMethod`, `PayMode`, `RulePriority`, `TargetPriority`.

- [ ] **Step 1: Write failing validation tests**

Add tests:

```go
func TestCreateRuleAcceptsMultipleAppsModesAndTargets(t *testing.T) {}
func TestCreateRuleRejectsInvalidPaymentMethod(t *testing.T) {}
func TestCreateRuleRejectsInvalidAppScope(t *testing.T) {}
func TestCreateRuleRejectsEmptyTargets(t *testing.T) {}
func TestCreateRuleRejectsInvalidTargetChannelAccountID(t *testing.T) {}
```

Run:

```bash
go test ./internal/domain/routing/test -run 'TestCreateRule' -count=1
```

Expected: FAIL until model is updated.

- [ ] **Step 2: Update repository**

Repository methods:

```go
func (r Repository) List(ctx context.Context) ([]RuleAggregate, error)
func (r Repository) FindByID(ctx context.Context, id int) (*RuleAggregate, error)
func (r Repository) Create(ctx context.Context, input RuleMutation) (*RuleAggregate, error)
func (r Repository) Update(ctx context.Context, id int, input RuleMutation) (*RuleAggregate, error)
func (r Repository) SetEnabled(ctx context.Context, id int, enabled bool) (*RuleAggregate, error)
```

`RuleAggregate`:

```go
type RuleAggregate struct {
	Rule *ent.RoutingRule
	Targets []*ent.RoutingTarget
}
```

Update implementation deletes existing targets and recreates submitted targets inside an Ent transaction.

- [ ] **Step 3: Update service validation**

Rules:

- `app_scope`: `all` or `include`; empty defaults to `all`.
- `app_ids`: trim entries, remove blanks, unique.
- `payment_method`: required and one of `wechat/alipay/paypal`.
- `pay_modes`: trim, lowercase, unique; empty means all modes for that method.
- `currency`: uppercase; empty means any currency supported by method.
- `targets`: at least one target required.
- target `channel_account_id` must be greater than 0.
- target `priority` default `100`.
- target `weight` default `100`.

- [ ] **Step 4: Verify validation tests**

Run:

```bash
go test ./internal/domain/routing/test -run 'TestCreateRule' -count=1
```

Expected: PASS.

### Task 3: Resolve Candidates Against Channel Accounts

**Files:**
- Modify: `internal/domain/channels/repository/repository.go`
- Modify: `internal/domain/routing/service/service.go`
- Test: `internal/domain/routing/test/service_test.go`

**Interfaces:**
- Add `channelrepo.Repository.FindEnabledByID(ctx context.Context, id int) (*ent.ChannelAccount, error)`.
- Routing service uses channel repository to validate targets during resolve.

- [ ] **Step 1: Write failing resolve tests**

Add tests:

```go
func TestResolveMatchesMultipleAppsAndPayModes(t *testing.T) {}
func TestResolveSkipsTargetWhenChannelAccountDisabled(t *testing.T) {}
func TestResolveSkipsTargetWhenAccountChannelDoesNotMatchPaymentMethod(t *testing.T) {}
func TestResolveSortsByRulePriorityThenTargetPriority(t *testing.T) {}
```

Run:

```bash
go test ./internal/domain/routing/test -run 'TestResolve' -count=1
```

Expected: FAIL until resolve is updated.

- [ ] **Step 2: Implement channel account lookup**

Add to `internal/domain/channels/repository/repository.go`:

```go
func (r Repository) FindEnabledByID(ctx context.Context, id int) (*ent.ChannelAccount, error) {
	return r.client.ChannelAccount.Query().
		Where(channelaccount.ID(id), channelaccount.Enabled(true)).
		First(ctx)
}
```

- [ ] **Step 3: Update service constructor**

```go
type Service struct {
	rules routingrepo.Repository
	channels channelrepo.Repository
}

func New(client *ent.Client) Service {
	return Service{
		rules: routingrepo.New(client),
		channels: channelrepo.New(client),
	}
}
```

- [ ] **Step 4: Implement resolve**

Matching:

- enabled rule only.
- `app_scope=all` always matches; `include` requires `input.AppID` in `rule.AppIDs`.
- `payment_method` equals input payment method.
- `pay_modes` empty or contains input pay mode.
- currency empty or equals input currency.
- amount and terminal match.
- payment method supports currency.

Target filtering:

- target enabled.
- channel account exists and enabled.
- account `Channel` equals rule `PaymentMethod`.

Sort:

1. rule priority desc
2. target priority desc
3. target id asc

- [ ] **Step 5: Verify resolve tests**

Run:

```bash
go test ./internal/domain/routing/test -run 'TestResolve' -count=1
```

Expected: PASS.

### Task 4: Update Checkout Routing Flow

**Files:**
- Modify: `internal/domain/orders/handler/checkout_handler.go`
- Modify: `internal/domain/orders/test/checkout_payment_test.go`
- Modify: `internal/domain/payments/service/service.go`
- Modify: `internal/domain/payments/test/provider_selection_test.go`

**Interfaces:**
- Checkout detects or receives `payment_method` and `pay_mode`.
- `paymentsvc.StartPaymentInput` adds `ChannelAccountID int`.
- Payment service loads selected account by ID when provided.

- [ ] **Step 1: Write failing checkout tests**

Add tests:

```go
func TestCheckoutHandlerRoutesToSpecificChannelAccount(t *testing.T) {}
func TestCheckoutHandlerStartPaymentUsesRoutedChannelAccount(t *testing.T) {}
func TestCheckoutHandlerReturnsNoMethodsWhenAllTargetsDisabled(t *testing.T) {}
func TestCheckoutHandlerFallsBackWhenNoRoutingRulesExist(t *testing.T) {}
```

Run:

```bash
go test ./internal/domain/orders/test -run 'TestCheckoutHandler.*Rout' -count=1
```

Expected: FAIL until checkout and payment service are updated.

- [ ] **Step 2: Update payment service selected account support**

Add field:

```go
ChannelAccountID int
```

Selection:

- if `ChannelAccountID > 0`, call `FindEnabledByID`.
- otherwise existing `FindEnabledByChannel` behavior remains.

- [ ] **Step 3: Update checkout method output**

Methods include:

```json
{
  "payment_method": "wechat",
  "pay_method": "wechat",
  "pay_mode": "native",
  "channel": "wechat",
  "channel_account_id": 1,
  "rule_id": 1,
  "target_id": 1,
  "label": "微信支付"
}
```

- [ ] **Step 4: Update start payment request**

Frontend `POST /orders/:id/pay` should send `channel_account_id` when a routed method is chosen.

Backend validates:

- provided account is enabled.
- account channel matches payment method.
- account supports currency.

- [ ] **Step 5: Verify checkout tests**

Run:

```bash
go test ./internal/domain/orders/test ./internal/domain/payments/test -count=1
```

Expected: PASS.

### Task 5: Update Admin Handler API

**Files:**
- Modify: `internal/domain/routing/handler/handler.go`
- Modify: `internal/domain/routing/test/handler_test.go`

**Interfaces:**
- Handler accepts nested `targets`.
- Preview accepts `payment_method` and `pay_mode`.

- [ ] **Step 1: Write failing handler tests**

Add tests:

```go
func TestRoutingHandlerCreatesRuleWithTargets(t *testing.T) {}
func TestRoutingHandlerPreviewsSpecificAccountCandidate(t *testing.T) {}
func TestRoutingHandlerRejectsRuleWithoutTargets(t *testing.T) {}
```

Run:

```bash
go test ./internal/domain/routing/test -run 'TestRoutingHandler' -count=1
```

Expected: FAIL until handler is updated.

- [ ] **Step 2: Update request structs**

```go
type manageRuleRequest struct {
	Name string
	Enabled *bool
	Priority int
	AppScope string
	AppIDs []string
	PaymentMethod string
	PayModes []string
	Currency string
	MinAmount int64
	MaxAmount int64
	Terminal string
	Targets []manageTargetRequest
}
```

- [ ] **Step 3: Update preview request**

```go
type previewRequest struct {
	AppID string
	PaymentMethod string
	PayMode string
	Amount int64
	Currency string
	Terminal string
}
```

- [ ] **Step 4: Verify handler tests**

Run:

```bash
go test ./internal/domain/routing/test -count=1
```

Expected: PASS.

### Task 6: Update Frontend Routing UI

**Files:**
- Modify: `web/src/features/routing/types.ts`
- Modify: `web/src/features/routing/api.ts`
- Modify: `web/src/features/routing/routing-page.tsx`
- Modify: `web/src/features/routing/routing-form-page.tsx`
- Modify: `web/src/features/checkout/types.ts`
- Modify: `web/src/features/checkout/checkout-page.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Routing form can enter multiple app IDs.
- Routing form can choose multiple pay modes.
- Routing form can choose multiple target channel accounts.
- Checkout sends `channel_account_id` when selected method contains it.

- [ ] **Step 1: Update TypeScript types**

`RoutingRule`:

```ts
type RoutingRule = {
  app_scope: "all" | "include"
  app_ids: string[]
  payment_method: "wechat" | "alipay" | "paypal"
  pay_modes: string[]
  targets: RoutingTarget[]
}
```

- [ ] **Step 2: Update list table**

Columns:

- rule name/priority
- apps
- payment method
- pay modes
- currency/amount/terminal
- targets count and account names when available
- enabled
- actions

- [ ] **Step 3: Update form**

Use shadcn inputs/selects/checkboxes:

- app scope select.
- app IDs textarea with one app per line.
- payment method select.
- pay mode checkboxes per method.
- target channel account checklist filtered by payment method.
- priority/amount/terminal/enabled.

- [ ] **Step 4: Update checkout selected method payload**

When selected method includes `channel_account_id`, include it in `POST /pay` body.

- [ ] **Step 5: Verify frontend**

Run:

```bash
make web-typecheck
cd web && bun run build
```

Expected: PASS.

### Task 7: Full Verification and Backend Restart

**Files:**
- No source files unless verification reveals compile failures.

- [ ] **Step 1: Run all backend tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend checks**

Run:

```bash
make web-typecheck
cd web && bun run build
```

Expected: PASS.

- [ ] **Step 3: Build backend**

Run:

```bash
go build -o .tmp/payment-gateway-server ./cmd/server
```

Expected: binary builds successfully.

- [ ] **Step 4: Restart backend**

Run:

```bash
ps -ef | rg 'payment-gateway-server|cmd/server' | rg -v rg
kill <pid>
set -a && . ./.env && set +a && ./.tmp/payment-gateway-server
```

- [ ] **Step 5: Smoke test**

Run:

```bash
curl -sS http://127.0.0.1:8080/healthz
```

Expected:

```json
{"code":"ok","data":{"status":"ok"},"error":null,"message":"ok"}
```

## Self-Review

- Spec coverage: covers multi-app rules, multiple pay modes, multiple channel-account targets, checkout integration, payment service selected-account support, admin API, frontend UI, tests, and restart.
- Placeholder scan: no `TBD`, `TODO`, or unspecified future work appears in executable tasks.
- Type consistency: `payment_method`, `pay_mode`, `channel_account_id`, `routing_rule_id`, and `routing_target_id` are consistent across backend, API, checkout, and frontend.
- Scope control: this plan implements deterministic target selection by priority. Weighted random, health scoring, automatic failover after provider errors, and routing analytics remain outside this slice.
