# shadcn-admin Frontend Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the payment gateway's administrative frontend shell and shared interaction system with the shadcn-admin design while preserving every existing route, API contract, and business workflow.

**Architecture:** Keep the existing Rsbuild entrypoint, TanStack Router route tree, API modules, Zustand auth store, i18n resources, and feature pages. Introduce a shadcn-admin-inspired authenticated layout, navigation metadata, command menu, page container/status primitives, and upgraded data-table utilities; adapt existing features to these shared primitives instead of copying upstream demo business code.

**Tech Stack:** React 19, TypeScript, Rsbuild, Tailwind CSS v4, shadcn/ui, Radix UI, TanStack Router/Table/Query, Zustand, i18next, `cmdk`, Lucide icons.

## Global Constraints

- Preserve all existing URLs and backend API behavior, including `/checkout/*` pages.
- Keep the project on Rsbuild; do not add Clerk or replace the existing authentication flow.
- All new user-facing text must be added to `web/src/i18n/locales/en.json` and `web/src/i18n/locales/zh-CN.json`.
- All business tables must continue to use `web/src/components/data-table/`.
- Preserve Apache-2.0 for this repository and add the upstream MIT notice to `web/THIRD_PARTY_NOTICES.md`.
- Run `bun run typecheck`, `bun run lint`, `bun run build`, and `bun test` in `web/` after frontend changes.

---

### Task 1: Align frontend dependencies and third-party attribution

**Files:**
- Modify: `web/package.json`
- Modify: `web/bun.lock`
- Create: `web/THIRD_PARTY_NOTICES.md`
- Test: `web/test/dependency-contract.test.mts`

**Interfaces:**
- Produces the `cmdk` dependency used by the command palette and a documented MIT notice for `satnaing/shadcn-admin`.

- [ ] **Step 1: Add only required shared dependencies**

Add `cmdk` and any missing Radix primitives required by the copied/adapted shadcn-admin components. Do not add Clerk, Vite, or duplicate React/TanStack packages.

- [ ] **Step 2: Update the lockfile**

Run `bun install --cwd web` and verify the lockfile resolves one version of each React and TanStack package family.

- [ ] **Step 3: Add attribution**

Create `web/THIRD_PARTY_NOTICES.md` with the upstream URL, MIT copyright notice for Sat Naing, and the full MIT permission/warranty text.

- [ ] **Step 4: Add the dependency contract test**

```ts
import { expect, test } from 'bun:test'

test('frontend uses shadcn-admin command palette dependency without Clerk', async () => {
  const packageJson = await Bun.file(new URL('../package.json', import.meta.url)).json()
  expect(packageJson.dependencies.cmdk).toBeDefined()
  expect(packageJson.dependencies['@clerk/react']).toBeUndefined()
})
```

- [ ] **Step 5: Run the test and commit**

Run: `cd web && bun test test/dependency-contract.test.mts`
Expected: PASS.

Commit: `git add web/package.json web/bun.lock web/THIRD_PARTY_NOTICES.md web/test/dependency-contract.test.mts && git commit -m "Add shadcn admin frontend dependencies"`

### Task 2: Create navigation metadata and layout primitives

**Files:**
- Create: `web/src/config/navigation.ts`
- Create: `web/src/components/layout/main.tsx`
- Create: `web/src/components/layout/app-sidebar.tsx`
- Create: `web/src/components/layout/header.tsx`
- Create: `web/src/components/layout/nav-user.tsx`
- Create: `web/src/components/layout/types.ts`
- Create: `web/src/components/skip-to-main.tsx`
- Test: `web/test/navigation.test.mts`

**Interfaces:**
- `navigation.ts` exports `sidebarData` and `getNavigationItems(t)`.
- `Main` accepts `{ fixed?: boolean; fluid?: boolean; className?: string }` and renders a semantic `<main id="main-content">`.
- `AppSidebar` consumes `SidebarData` and the existing `useAuthStore` user/logout behavior.

- [ ] **Step 1: Define typed navigation metadata**

Define `NavItem`, `NavGroup`, and `SidebarData` with `titleKey`, `url`, `icon`, optional `disabled`, and optional `match` fields. Include every existing admin URL and no checkout route.

- [ ] **Step 2: Implement the main container and skip link**

Implement `Main` using responsive shadcn-admin spacing and `SkipToMain` targeting `#main-content`; both components must be keyboard accessible.

- [ ] **Step 3: Implement sidebar and user menu**

Use the existing `Sidebar` primitives, translated labels, active route matching, current user avatar fallback, and `logout()`/`clearSession()` behavior. Keep mobile drawer and collapsed icon states.

- [ ] **Step 4: Test navigation coverage**

```ts
import { expect, test } from 'bun:test'
import { navigationUrls } from '../src/config/navigation'

test('navigation covers every administrative entry point', () => {
  expect(navigationUrls).toEqual(expect.arrayContaining(['/','/apps','/orders','/webhooks','/refunds','/reconciliations','/channels','/routing','/config/gateway','/users']))
  expect(navigationUrls.some((url) => url.startsWith('/checkout'))).toBe(false)
})
```

- [ ] **Step 5: Run checks and commit**

Run: `cd web && bun test test/navigation.test.mts && bun run typecheck`
Expected: PASS.

Commit: `git add web/src/config web/src/components/layout web/src/components/skip-to-main.tsx web/test/navigation.test.mts && git commit -m "Add shadcn admin layout primitives"`

### Task 3: Replace the authenticated root shell

**Files:**
- Modify: `web/src/routes/root.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/theme-provider.tsx`
- Modify: `web/src/hooks/use-document-title.ts`
- Test: `web/test/layout-auth.test.mts`

**Interfaces:**
- `ShellLayout` keeps the existing `rootRoute` export and renders `Outlet` through `AuthenticatedLayout` for admin URLs.
- Checkout routes bypass the authenticated shell exactly as before.

- [ ] **Step 1: Move shell markup into layout components**

Replace the monolithic `ShellLayout` markup with `SidebarProvider`, `AppSidebar`, `Header`, and `Main`. Preserve active route detection and page title lookup.

- [ ] **Step 2: Add header actions**

Add breadcrumb, theme switch, language switch, command menu trigger, notification action, and user menu. Every action must have translated accessible labels.

- [ ] **Step 3: Preserve auth and checkout branching**

Keep `AuthScreen` rendering for missing access tokens and keep `/checkout/*` outside the management shell.

- [ ] **Step 4: Add layout regression tests**

Test that the shell exports the same root route, contains `#main-content`, and the route predicate excludes `/checkout/order/result` from the admin chrome.

- [ ] **Step 5: Run checks and commit**

Run: `cd web && bun test test/layout-auth.test.mts && bun run typecheck && bun run build`
Expected: PASS.

Commit: `git add web/src/routes/root.tsx web/src/App.tsx web/src/components/theme-provider.tsx web/src/hooks/use-document-title.ts web/test/layout-auth.test.mts && git commit -m "Replace admin root shell with shadcn layout"`

### Task 4: Add command menu, theme controls, and error states

**Files:**
- Create: `web/src/components/command-menu.tsx`
- Create: `web/src/components/theme-switch.tsx`
- Create: `web/src/components/language-switch.tsx`
- Create: `web/src/components/page-state.tsx`
- Create: `web/src/routes/errors.tsx`
- Modify: `web/src/router.tsx`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Test: `web/test/command-menu.test.mts`

**Interfaces:**
- `CommandMenu` reads the shared navigation metadata and navigates with TanStack Router.
- `PageState` exposes `loading`, `empty`, and `error` variants with translated copy and retry callback.

- [ ] **Step 1: Implement command menu**

Use `cmdk`, `⌘K`/`Ctrl+K`, navigation groups, theme commands, and a close-on-navigation callback. Do not include disabled subscription items as executable commands.

- [ ] **Step 2: Implement theme and language switches**

Theme switch supports light/dark/system through the existing provider. Language switch calls `i18n.changeLanguage` and persists through the existing detector.

- [ ] **Step 3: Implement reusable page states**

Use Skeleton, Alert, Button, and Empty card primitives; no page may invent a different loading or retry treatment after this task.

- [ ] **Step 4: Register error routes and translations**

Add 401, 403, 404, 500 route components and translation keys in both locale files without changing API error payloads.

- [ ] **Step 5: Test keyboard and route behavior**

```ts
import { expect, test } from 'bun:test'

test('command menu exposes keyboard shortcut and admin routes', () => {
  const source = Bun.file(new URL('../src/components/command-menu.tsx', import.meta.url))
  expect(source).toBeDefined()
})
```

Run: `cd web && bun test test/command-menu.test.mts && bun run typecheck`
Expected: PASS.

Commit: `git add web/src/components/command-menu.tsx web/src/components/theme-switch.tsx web/src/components/language-switch.tsx web/src/components/page-state.tsx web/src/routes/errors.tsx web/src/router.tsx web/src/i18n/locales && git commit -m "Add shadcn admin command and error states"`

### Task 5: Upgrade shared data-table infrastructure

**Files:**
- Modify: `web/src/components/data-table/data-table.tsx`
- Modify: `web/src/components/data-table/factory.tsx`
- Modify: `web/src/components/data-table/index.ts`
- Create: `web/src/components/data-table/toolbar.tsx`
- Create: `web/src/components/data-table/view-options.tsx`
- Create: `web/src/components/data-table/column-header.tsx`
- Create: `web/src/components/data-table/pagination.tsx`
- Test: `web/test/data-table.test.mts`

**Interfaces:**
- Existing `createDataTable<T>()` remains source-compatible for all current feature pages.
- New optional props are `toolbar`, `pagination`, `viewOptions`, and `emptyState`; defaults preserve current behavior.

- [ ] **Step 1: Preserve factory generics**

Keep `createDataTable<T>()` and `DataTableColumn<T>` exports unchanged; add optional UI slots rather than changing required props.

- [ ] **Step 2: Add shadcn-admin toolbar and view controls**

Implement reusable search/filter/action controls and column visibility using TanStack Table state. Keep all labels translated by the caller.

- [ ] **Step 3: Add responsive pagination**

Use the current pagination state and existing API page/size values; do not introduce client-side pagination for server-paginated endpoints.

- [ ] **Step 4: Add compatibility test**

Verify a minimal `createDataTable<{ id: string }>()` invocation still typechecks and renders headers, rows, empty state, and row actions.

- [ ] **Step 5: Run checks and commit**

Run: `cd web && bun test test/data-table.test.mts && bun run typecheck && bun run build`
Expected: PASS.

Commit: `git add web/src/components/data-table web/test/data-table.test.mts && git commit -m "Upgrade data tables for shadcn admin"`

### Task 6: Migrate dashboard and business pages to shared layout

**Files:**
- Modify: `web/src/routes/index.tsx`
- Modify: `web/src/features/apps/apps-page.tsx`
- Modify: `web/src/features/orders/orders-page.tsx`
- Modify: `web/src/features/webhooks/webhooks-page.tsx`
- Modify: `web/src/features/refunds/refunds-page.tsx`
- Modify: `web/src/features/reconciliations/reconciliations-page.tsx`
- Modify: `web/src/features/channels/channels-page.tsx`
- Modify: `web/src/features/routing/routing-page.tsx`
- Modify: `web/src/features/users/users-page.tsx`
- Modify: `web/src/features/config/gateway-config-page.tsx`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Test: existing page tests plus `web/test/admin-route-smoke.test.mts`

**Interfaces:**
- Feature API modules and exported page components remain unchanged.
- Each page uses `PageState`, `Main`, shared page heading, and shared Data Table controls.

- [ ] **Step 1: Add page heading/container pattern**

Use a consistent title, description, primary action, and responsive content grid on dashboard and list pages.

- [ ] **Step 2: Migrate list pages in payment order**

Update orders, webhooks, refunds, and reconciliations first; preserve filters, URL state, row actions, detail links, retry actions, and confirmation dialogs.

- [ ] **Step 3: Migrate platform pages**

Update apps, channels, routing, gateway config, and users; preserve create/edit forms, validation, permission-sensitive actions, and detail navigation.

- [ ] **Step 4: Refresh dashboard cards and charts**

Use shadcn-admin card spacing and responsive chart containers while retaining existing monitoring API data and Webhook delivery table.

- [ ] **Step 5: Add route smoke coverage**

Assert all administrative route modules still resolve and each page references the shared page/table primitives.

- [ ] **Step 6: Run checks and commit**

Run: `cd web && bun test && bun run typecheck && bun run lint && bun run build`
Expected: PASS.

Commit: `git add web/src/routes/index.tsx web/src/features web/src/i18n/locales web/test/admin-route-smoke.test.mts && git commit -m "Migrate payment pages to shadcn admin layout"`

### Task 7: Adapt checkout and test-payment layouts

**Files:**
- Modify: `web/src/routes/checkout.tsx`
- Modify: `web/src/routes/test-pay.tsx`
- Modify: `web/src/features/checkout/checkout-page.tsx`
- Modify: `web/src/features/checkout/checkout-result-page.tsx`
- Modify: `web/src/features/test-pay/test-pay-page.tsx`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Test: existing checkout tests and `web/test/checkout-layout.test.mts`

**Interfaces:**
- Checkout URLs, query parameters, payment initiation, polling, result handling, and test-pay actions remain unchanged.

- [ ] **Step 1: Keep checkout outside admin chrome**

Use a public responsive shell with shadcn-admin tokens, logo/product identity, language switch, and compact theme control; do not render admin sidebar or authenticated user menu.

- [ ] **Step 2: Apply shared states and form primitives**

Use the new `PageState` and shadcn form/card/button primitives without changing payment request payloads.

- [ ] **Step 3: Verify payment workflows**

Run existing checkout tests and manually verify successful, failed, expired, and invalid-order states.

- [ ] **Step 4: Commit**

Run: `cd web && bun test && bun run typecheck && bun run build`
Expected: PASS.

Commit: `git add web/src/routes/checkout.tsx web/src/routes/test-pay.tsx web/src/features/checkout web/src/features/test-pay web/src/i18n/locales web/test/checkout-layout.test.mts && git commit -m "Apply shadcn styling to checkout flows"`

### Task 8: Normalize styles, docs, and release verification

**Files:**
- Modify: `web/src/styles/index.css`
- Modify: `web/src/styles/shadcn.css`
- Modify: `web/src/App.tsx`
- Modify: `web/README.md`
- Modify: root `README.md`
- Test: `web/test/api-base-url.test.mts`, `web/test/refund-and-reconciliation-filters.test.mts`, `web/test/webhook-event-types.test.mts`

**Interfaces:**
- No public runtime interface changes; this task removes duplicate styles and documents the new frontend architecture.

- [ ] **Step 1: Consolidate design tokens**

Move the final shadcn-admin-compatible color, radius, typography, focus-ring, sidebar, and chart tokens into `index.css`; remove duplicate declarations from `shadcn.css` only after all imports compile.

- [ ] **Step 2: Verify responsive and accessibility behavior**

Run the app in desktop and mobile viewport sizes; verify keyboard navigation, focus visibility, sidebar drawer, skip link, reduced motion, and dark mode.

- [ ] **Step 3: Update frontend documentation**

Document layout ownership, navigation metadata, shared page states, Data Table usage, and the upstream MIT notice in `web/README.md`; add the shadcn-admin attribution link to the root README technology notes.

- [ ] **Step 4: Run the complete verification gate**

Run:

```bash
cd web
bun test
bun run typecheck
bun run lint
bun run format:check
bun run build
```

Expected: all commands exit with status 0 and the production bundle is generated by Rsbuild.

- [ ] **Step 5: Commit the final migration**

Commit: `git add web/src/styles web/src/App.tsx web/README.md README.md web/test && git commit -m "Finalize shadcn admin frontend migration"`

After the branch passes the verification gate, push it and open a PR for protected-branch review; do not push directly to `main`.
