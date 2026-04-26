.PHONY: help bootstrap up down logs psql migrate-up migrate-down migrate-status \
        gen-sqlc gen-api gen api web format test lint typecheck

help:
	@echo "Cadence — common make targets"
	@echo ""
	@echo "  bootstrap         Install all dependencies (pnpm + go work sync)"
	@echo "  up                Start Postgres (docker compose)"
	@echo "  down              Stop docker compose services"
	@echo "  logs              Tail docker compose logs"
	@echo "  psql              Open psql shell against local DB"
	@echo ""
	@echo "  migrate-up        Apply pending goose migrations"
	@echo "  migrate-down      Roll back one goose migration"
	@echo "  migrate-status    Show migration status"
	@echo ""
	@echo "  gen-sqlc          Regenerate sqlc Go code from queries"
	@echo "  gen-api           Regenerate orval TS client from openapi.yaml"
	@echo "  gen               Run all codegen"
	@echo ""
	@echo "  api               Run apps/api with hot-reload (HTTP server + River worker)"
	@echo "  web               Run apps/web (Next.js dev server)"
	@echo ""
	@echo "  format            Format all code (biome + gofmt)"
	@echo "  lint              Lint all code"
	@echo "  typecheck         Run typecheck across all TS packages"
	@echo "  test              Run all tests"

bootstrap:
	pnpm install
	go work sync

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

psql:
	docker compose exec postgres psql -U cadence -d cadence

migrate-up:
	cd apps/api && goose -dir internal/db/migrations postgres "$$DATABASE_URL" up

migrate-down:
	cd apps/api && goose -dir internal/db/migrations postgres "$$DATABASE_URL" down

migrate-status:
	cd apps/api && goose -dir internal/db/migrations postgres "$$DATABASE_URL" status

gen-sqlc:
	cd apps/api && sqlc generate

gen-api:
	pnpm turbo gen:api

gen: gen-sqlc gen-api

api:
	cd apps/api && air -c .air.toml

web:
	pnpm --filter web dev

format:
	pnpm biome check --write .
	go fmt ./...

lint:
	pnpm turbo lint
	golangci-lint run ./...

typecheck:
	pnpm turbo typecheck

test:
	pnpm turbo test
	go test ./...
