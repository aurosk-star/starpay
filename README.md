# Payment Gateway

Internal payment gateway service for unified payment orders, channel routing, callbacks, webhooks, refunds, and later subscriptions.

## Stack

- Go + Gin for HTTP APIs
- Ent for database models
- PostgreSQL for persistent storage
- Redis for cache, locks, and async coordination
- Docker Compose for local dependencies

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
