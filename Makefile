COMPOSE_FILE := deploy/docker-compose/docker-compose.yml

.PHONY: build test lint run stop web-install web-build migrate-up migrate-down migrate-create migrate-version

DB_DSN ?= postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable
MIGRATE_PATH := services/shortener/migrations/postgres

## build: compile both binaries locally
build:
	go build ./services/shortener/cmd/shortener
	go build ./services/analytics/cmd/analytics

## test: run all tests
test:
	go test ./...

## lint: placeholder for linter (add golangci-lint later)
lint:
	go vet ./...

## run: start the full Docker Compose stack (prefer task deploy for the canonical local workflow)
run:
	docker compose -f $(COMPOSE_FILE) up --build -d

## stop: tear down the Docker Compose stack
stop:
	docker compose -f $(COMPOSE_FILE) down

## web-install: install frontend dependencies
web-install:
	cd services/web && npm ci

## web-build: build the Next.js frontend
web-build:
	cd services/web && npm run build

## migrate-up: apply all pending PostgreSQL migrations
migrate-up:
	migrate -path $(MIGRATE_PATH) -database "$(DB_DSN)" up

## migrate-down: roll back the last PostgreSQL migration
migrate-down:
	migrate -path $(MIGRATE_PATH) -database "$(DB_DSN)" down 1

## migrate-create: create a new migration pair (usage: make migrate-create NAME=add_users_table)
migrate-create:
	migrate create -ext sql -dir $(MIGRATE_PATH) -seq $(NAME)

## migrate-version: show current migration version
migrate-version:
	migrate -path $(MIGRATE_PATH) -database "$(DB_DSN)" version
