.PHONY: dev test tidy ent-up db-up db-down web-dev web-build web-typecheck

dev:
	go run ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

ent-up:
	go run entgo.io/ent/cmd/ent generate ./ent/schema

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose down

web-dev:
	cd web && bun run dev

web-build:
	cd web && bun run build

web-typecheck:
	cd web && bun run typecheck
