.PHONY: infra-up infra-down infra-status api-build api-test api-run bootstrap-admin migrate-up migrate-down web-install web-generate web-dev web-check build check

infra-up:
	docker compose -f deployments/dev/compose.yaml up -d

infra-down:
	docker compose -f deployments/dev/compose.yaml down

infra-status:
	docker compose -f deployments/dev/compose.yaml ps

api-build:
	cd apps/api && go build -o ../../bin/quickeval ./cmd/server

api-test:
	cd apps/api && go test ./...

api-run:
	cd apps/api && QUICKEVAL_BASE_DIR=../.. go run ./cmd/server

bootstrap-admin:
	cd apps/api && QUICKEVAL_BASE_DIR=../.. go run ./cmd/bootstrap-admin

migrate-up:
	cd apps/api && QUICKEVAL_BASE_DIR=../.. go run ./cmd/migrate -direction up

migrate-down:
	cd apps/api && QUICKEVAL_BASE_DIR=../.. go run ./cmd/migrate -direction down -steps 1

web-install:
	cd apps/web && npm install

web-generate:
	cd apps/web && npm run generate:api

web-dev:
	cd apps/web && npm run dev

web-check:
	cd apps/web && npm run check

build: api-build
	cd apps/web && npm run build

check: api-test web-check
