APP_NAME=trip-api
DATABASE_URL?=postgres://trip:trip@localhost:5432/tripdb?sslmode=disable
MIGRATIONS_PATH?=file://migrations

.PHONY: run build test swagger migrate-up migrate-down docker-up docker-down tidy

tidy:
	go mod tidy

build:
	go build -o bin/$(APP_NAME) ./cmd/api

run:
	DATABASE_URL=$(DATABASE_URL) MIGRATIONS_PATH=$(MIGRATIONS_PATH) go run ./cmd/api

test:
	go test ./... -count=1

swagger:
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs --parseDependency --parseInternal

migrate-up:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$(DATABASE_URL)" down 1

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
