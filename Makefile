APP_NAME=trip-api
DATABASE_URL?=postgres://trip:trip@localhost:5432/tripdb?sslmode=disable
MIGRATIONS_PATH?=file://migrations
GOLANGCI_LINT_VERSION?=v2.15.0
GOLANGCI_LINT:=$(shell command -v golangci-lint 2>/dev/null)

.PHONY: help tidy build run test test-cover vet lint lint-fix fmt hooks hooks-remove check swagger migrate-up migrate-down docker-up docker-down clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

tidy: ## Sync go.mod/go.sum
	go mod tidy

build: ## Build the API binary into bin/trip-api
	go build -o bin/$(APP_NAME) ./cmd/api

run: ## Run the API locally (requires DATABASE_URL)
	DATABASE_URL=$(DATABASE_URL) MIGRATIONS_PATH=$(MIGRATIONS_PATH) go run ./cmd/api

test: ## Run all tests without cache
	go test ./... -count=1

test-cover: ## Run tests and print per-function coverage
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (uses go run fallback if not installed)
	@if [ -n "$(GOLANGCI_LINT)" ]; then \
		"$(GOLANGCI_LINT)" run ./...; \
	else \
		echo "golangci-lint not found; running pinned $(GOLANGCI_LINT_VERSION) via go run"; \
		go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...; \
	fi

lint-fix: ## Apply golangci-lint autofixes
	@if [ -n "$(GOLANGCI_LINT)" ]; then \
		"$(GOLANGCI_LINT)" run --fix ./...; \
	else \
		go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --fix ./...; \
	fi

fmt: ## Format Go code (gofmt + goimports)
	gofmt -w .
	@if [ -n "$(GOLANGCI_LINT)" ]; then \
		"$(GOLANGCI_LINT)" fmt ./...; \
	else \
		go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) fmt ./...; \
	fi

hooks: ## Install the repo-local Git pre-commit hook
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "Git hooks enabled: core.hooksPath=.githooks"

hooks-remove: ## Remove the repo-local Git hook configuration
	-@git config --unset core.hooksPath
	@echo "Git hooks disabled"

check: vet lint test ## Run vet, lint, and tests

swagger: ## Regenerate Swagger docs from handler annotations
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs --parseDependency --parseInternal

migrate-up: ## Apply all pending migrations
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back one migration
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$(DATABASE_URL)" down 1

docker-up: ## Start the full stack in Docker
	docker compose up --build -d

docker-down: ## Stop the Docker stack
	docker compose down

clean: ## Remove build and coverage artifacts
	rm -rf bin coverage.out coverage.html
