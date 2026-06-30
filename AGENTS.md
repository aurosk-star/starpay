# Repository Guidelines

## Project Structure & Module Organization

This repository currently contains `PAYMENT_GATEWAY_PRD.md` and does not yet include application source code, tests, or build configuration. Treat the PRD as the source of truth for gateway scope, business rules, and planned capabilities.

Keep business code organized by domain under `internal/domain/`. Each domain owns its own `handler/`, `router/`, `service/`, `repository/`, and `test/` packages. Current domains are `apps`, `orders`, `channels`, `routing`, `webhooks`, `refunds`, and `subscriptions`.

Shared infrastructure belongs under `internal/platform/`: `config`, `database`, `cache`, `http`, and future cross-cutting packages such as `logger`. Ent schemas live under `ent/schema/`, generated Ent code lives under `ent/`, and the service entrypoint is `cmd/server/main.go`.

## Build, Test, and Development Commands

No build or test commands are defined yet. Add commands to the chosen stack's standard file, such as `package.json`, `Makefile`, `pyproject.toml`, or `go.mod`.

Document each new command here when it is introduced. Examples:

- `make test` or `npm test`: run tests.
- `make lint` or `npm run lint`: run style checks.
- `make dev` or `npm run dev`: start locally.

## Coding Style & Naming Conventions

Use clear module names that match gateway concepts: `payment_order`, `channel_route`, `webhook_delivery`, `refund_request`, and `subscription_plan`. Store money as integer minor units only, never floating point values. Prefer explicit status names such as `pending`, `succeeded`, `failed`, `refunded`, and `cancelled`.

Keep provider-specific code behind adapter interfaces so core order, routing, idempotency, and webhook behavior stays provider-neutral.

## Testing Guidelines

Add tests with every behavior change once implementation starts. Prioritize coverage for idempotent order creation, signature verification, channel callback validation, webhook retry behavior, refund state transitions, and currency handling.

Name tests after observable behavior, for example `test_create_order_returns_existing_order_for_same_request` or `webhook_delivery_retries_after_timeout`.

## Commit & Pull Request Guidelines

This directory is not currently a git repository, so no local commit history is available to infer conventions. Use concise, imperative commit messages such as `Add payment order idempotency checks`.

Pull requests should include a summary, test results, configuration changes, and related requirements or issues. For UI or admin-console changes, include screenshots.

## Security & Configuration Tips

Never commit channel secrets, app secrets, certificates, private keys, or webhook signing keys. Keep credentials in environment variables or a managed secret store. Payment callbacks and internal webhooks must verify signatures, enforce idempotency, and log enough context for reconciliation without exposing sensitive data.
