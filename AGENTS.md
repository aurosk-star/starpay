# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go payment gateway with a React admin console. The backend entrypoint is `cmd/server/main.go`. Domain code lives under `internal/domain/<module>/`, and each domain must own its `handler/`, `router/`, `service/`, `repository/`, and `test/` directories. Existing domains include `apps`, `users`, `channels`, `orders`, `routing`, `webhooks`, `refunds`, and `subscriptions`.

Shared backend infrastructure belongs under `internal/platform/`, including `config`, `database`, `cache`, `http`, `httpx`, `auth`, and `rbac`. Ent schemas live in `ent/schema/`; generated Ent code lives in `ent/`. The frontend lives in `web/`.

## Build, Test, and Development Commands

- `make dev`: run the Gin server from `cmd/server`.
- `make test`: run all Go tests with `go test ./...`.
- `make ent-up`: regenerate Ent code from `ent/schema/`.
- `make db-up`: start PostgreSQL and Redis with Docker Compose.
- `make db-down`: stop Docker Compose services.
- `make web-dev`: start the Rsbuild frontend with Bun.
- `make web-build`: build the frontend.
- `make web-typecheck`: run TypeScript checks.

Use `.env` for local service configuration. Docker Compose uses latest PostgreSQL and Redis images by project decision.

After any backend code change, build the latest backend binary and restart the running backend service from that build before handing off for testing. Do not leave an older `go run` or stale binary process serving requests after backend edits.

## Coding Style & Naming Conventions

Backend code uses Gin, Ent, PostgreSQL, Redis, Casbin, and the global response helpers in `internal/platform/httpx`. All API responses must use the shape `{ code, message, data, error }`; do not write raw `ctx.JSON` response bodies in handlers.

Keep business behavior inside the domain service layer, persistence in repository packages, HTTP parsing/serialization in handlers, and route registration in routers. Store money as integer minor units only. Use explicit status names such as `pending`, `succeeded`, `failed`, `refunded`, `cancelled`, `enabled`, and `disabled`.

Frontend work must use shadcn/ui as the primary component system. Use the configured shadcn preset, default component styles, Tailwind semantic tokens, and `web/src/components/theme-provider.tsx` for light/dark mode. The `d` key toggles color mode. Default UI language is Chinese, and new text must be added to i18n resources.

All frontend data displays must use the reusable Data Table factory in `web/src/components/data-table/`; business pages should not hand-roll table markup.

## Testing Guidelines

Add tests with every behavior change. Backend tests belong inside each module's `test/` directory, for example `internal/domain/apps/test/`. Prefer external test package names such as `appstest` or `userstest` so tests exercise exported behavior.

Prioritize tests for app credential generation, app status changes, idempotent order creation, signature verification, channel callbacks, webhook retry behavior, refund state transitions, and currency handling.

## Commit & Pull Request Guidelines

Use concise, imperative commit messages matching the existing history, such as `Add admin auth and RBAC foundation` or `Restructure code by domain modules`.

Pull requests should include a summary, test results, configuration changes, and related PRD requirements or issues. UI changes should include screenshots or a short visual verification note.

## Security & Configuration Tips

Never commit channel secrets, app secrets, certificates, private keys, webhook signing keys, or real credentials. Store secrets in environment variables or a managed secret store. Application secrets should be hashed at rest, shown only once after generation or reset, and protected by admin RBAC.
