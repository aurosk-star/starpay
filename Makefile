.PHONY: dev test sdk-test web-test web-admin-test deploy-test security verify tidy ent-up db-up db-down compose-up compose-down compose-logs web-dev web-build web-typecheck web-admin-dev web-admin-build web-admin-typecheck web-legacy-dev web-legacy-build web-legacy-typecheck

dev:
	go run ./cmd/server

test:
	go test ./...
	cd sdk/go && go test -count=1 ./...

sdk-test:
	cd sdk/go && go test -count=1 ./...

web-test:
	cd web && bun test test/*.test.mts

web-admin-test:
	cd web-admin && bun run typecheck && bun run build

deploy-test:
	bash scripts/deploy_test.sh

security:
	go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd web && bun audit

verify: test web-test deploy-test web-typecheck web-build security
	go vet ./...
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
	cd web-admin && bun run dev

web-build:
	cd web-admin && bun run build

web-typecheck:
	cd web-admin && bun run typecheck

web-admin-dev:
	cd web-admin && bun run dev

web-admin-build:
	cd web-admin && bun run build

web-admin-typecheck:
	cd web-admin && bun run typecheck

web-legacy-dev:
	cd web && bun run dev

web-legacy-build:
	cd web && bun run build

web-legacy-typecheck:
	cd web && bun run typecheck
