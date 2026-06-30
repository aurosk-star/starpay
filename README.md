# Payment Gateway

Internal payment gateway service for unified payment orders, channel routing, callbacks, webhooks, refunds, and later subscriptions.

## Stack

- Go + Gin for HTTP APIs
- Ent for database models
- PostgreSQL for persistent storage
- Redis for cache, locks, and async coordination
- React + Rsbuild + TanStack Router for the admin frontend
- Docker Compose for local dependencies

## Project Layout

- `cmd/server/`: service entrypoint
- `web/`: frontend admin console
- `internal/domain/`: payment gateway business domains, each with `handler/`, `router/`, `service/`, `repository/`, and `test/`
- `internal/platform/`: shared infrastructure packages
- `ent/schema/`: Ent schema definitions
- `ent/`: generated Ent code

## Local Development

```bash
cp .env.example .env
make db-up
make ent-up
make dev
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Run tests:

```bash
make test
```

Frontend:

```bash
cd web
bun install
bun run dev
bun run build
```
