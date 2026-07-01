# Configurable Test Payment Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin test payment page where operators can create checkout test orders with editable parameters, currency, channel, and return URL.

**Architecture:** Create a frontend-only test tool under `web/src/features/test-pay/` that calls the existing admin order creation API, then routes to `/checkout/{gateway_order_no}`. Add a TanStack route `/test-pay` and change the dashboard test payment button to navigate there instead of creating a hardcoded order.

**Tech Stack:** React, TanStack Router, shadcn/ui, TypeScript, existing `createTestCheckoutOrder` API.

## Global Constraints

- UI must use shadcn/ui components as the primary component system.
- Default UI language is Chinese; add all visible text to i18n resources.
- Do not change payment provider behavior or backend payment flow.
- Frontend data displays continue using existing abstractions; this page is a form tool, not a table.

---

### Task 1: Add Test Pay Page Component

**Files:**
- Create: `web/src/features/test-pay/test-pay-page.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes: `createTestCheckoutOrder(accessToken, payload)`
- Produces: `TestPayPage` React component

- [ ] **Step 1: Build a shadcn form**

Use `Card`, `FieldGroup`, `Field`, `FieldLabel`, `FieldDescription`, `Input`, `Textarea`, `Select`, `Button`, `Alert`, and `Separator`.

Fields:
- `app_id`, default `snsgo`
- `merchant_order_no`, default `test_checkout_${Date.now()}`
- `business_type`, default `checkout_test`
- `subject`
- `description`
- `amountMajor`, default `0.99`
- `currency`, default `USD` for PayPal and `CNY` otherwise
- `channel`, default `paypal`
- `pay_method`, default same as channel
- `return_url`
- `metadataJson`

- [ ] **Step 2: Submit behavior**

On submit:
- require access token
- parse amount major into minor units
- parse metadata JSON, default to `{}`
- call `createTestCheckoutOrder`
- navigate to `/checkout/{gateway_order_no}`

### Task 2: Add Route and Dashboard Link

**Files:**
- Create: `web/src/routes/test-pay.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/routes/index.tsx`

**Interfaces:**
- Consumes: `TestPayPage`
- Produces: route `/test-pay`

- [ ] **Step 1: Add `testPayRoute`**

Use `createRoute({ path: "/test-pay" })`.

- [ ] **Step 2: Register route**

Add `testPayRoute` to `routeTree`.

- [ ] **Step 3: Change homepage button**

Replace direct hardcoded order creation with `router.navigate({ to: "/test-pay" })`.

### Task 3: Verify

**Files:**
- Modify: none

**Interfaces:**
- Consumes: frontend changes
- Produces: passing typecheck/build

- [ ] **Step 1: Typecheck**

Run: `cd web && bun run typecheck`

Expected: PASS.

- [ ] **Step 2: Build**

Run: `cd web && bun run build`

Expected: PASS.
