# Orders Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin orders page that lists payment orders with filters, pagination, and safe lifecycle actions.

**Architecture:** Reuse the existing React + Rsbuild + shadcn/ui console patterns already used by apps and channels. The page should be read-first, with a Data Table for order rows, a filter bar for app/status/channel/currency/order number, and a detail drawer or page only if needed later.

**Tech Stack:** React 19, TanStack Router, shadcn/ui, lucide-react, i18next, local Data Table factory.

## Global Constraints

- Use shadcn/ui as the primary component system.
- All tabular data must use `web/src/components/data-table/`.
- Default UI language is Chinese.
- Use the existing theme provider and sidebar shell.
- Keep the first version aligned to the admin backend endpoints under `/v1/admin/orders`.

---

### Task 1: Orders API And Types

**Files:**
- Create: `web/src/features/orders/api.ts`
- Create: `web/src/features/orders/types.ts`

**Interfaces:**
- Produces: `PaymentOrder`, `ListOrdersResponse`, and request helpers for list/detail/create/update/close.

- [ ] Write the failing TypeScript usage test or type-checkable component stub that imports the new orders API and types.
- [ ] Define the API functions for `listOrders`, `getOrder`, `createOrder`, `updateOrder`, and `closeOrder`.
- [ ] Define the shared order types with fields that match the backend response shape.
- [ ] Run `bun run typecheck` for the web package and confirm the new module compiles.

### Task 2: Orders List Page

**Files:**
- Create: `web/src/features/orders/orders-page.tsx`
- Create: `web/src/routes/orders.tsx`
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Consumes: `listOrders(accessToken, params)`.
- Produces: `/orders` admin route with Chinese labels, filters, refresh, and Data Table rendering.

- [ ] Add the page using `createDataTable` with columns for gateway order number, app, merchant order number, amount, currency, status, channel, and timestamps.
- [ ] Add filter controls for app ID, status, channel, currency, and merchant order number.
- [ ] Add row actions for view detail and close order where appropriate.
- [ ] Register the route in the shell and add the sidebar label translation.
- [ ] Run the web app typecheck and build checks.

### Task 3: Order Detail And Safe Actions

**Files:**
- Create: `web/src/features/orders/order-detail-page.tsx`
- Modify: `web/src/features/orders/api.ts`
- Modify: `web/src/routes/orders.tsx`

**Interfaces:**
- Consumes: `getOrder` and `closeOrder`.
- Produces: a detail view that shows payment lifecycle fields without exposing unsafe edits.

- [ ] Add a read-only detail view for one order.
- [ ] Add a close action with confirmation using shadcn `AlertDialog`.
- [ ] Keep create/edit behavior out of the first pass unless the backend exposes safe admin forms later.
- [ ] Run a focused web typecheck and manual route check.

### Task 4: Console Polish And Verification

**Files:**
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/i18n/resources.ts`

**Interfaces:**
- Produces: consistent sidebar label, breadcrumb, and localized order copy.

- [ ] Add the Orders nav item to the sidebar with the existing icon style.
- [ ] Ensure the top bar breadcrumb resolves correctly on the orders route.
- [ ] Run `bun run typecheck` and `bun run build`.
- [ ] Capture the page behavior for handoff.
