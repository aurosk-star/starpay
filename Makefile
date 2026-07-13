.PHONY: dev test sdk-test web-test deploy-test verify tidy ent-up db-up db-down compose-up compose-down compose-logs web-dev web-build web-typecheck

dev:
	go run ./cmd/server

test:
	go test ./...
	cd sdk/go && go test -count=1 ./...

sdk-test:
	cd sdk/go && go test -count=1 ./...

web-test:
	cd web && node --test test/*.test.mts

deploy-test:
	bash scripts/deploy_test.sh

verify: test web-test deploy-test web-typecheck web-build
	cd sdk/go && go vet ./...
	cd web && bun run lint

tidy:
	go mod tidy

ent-up:
	go run entgo.io/ent/cmd/ent generate ./ent/schema

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose down

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f api

web-dev:
	cd web && bun run dev

web-build:
	cd web && bun run build

web-typecheck:
	cd web && bun run typecheck
