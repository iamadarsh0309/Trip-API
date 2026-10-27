# Trip Management API

[![CI](https://github.com/iamadarsh0309/Trip-API/actions/workflows/ci.yml/badge.svg)](https://github.com/iamadarsh0309/Trip-API/actions/workflows/ci.yml)

Production-oriented Go (Gin) REST service for managing trips with:

- PostgreSQL persistence and versioned schema migrations
- Idempotent `POST /trips` via the `Idempotency-Key` header
- Enforced status transitions and safe concurrent status updates (`SELECT ... FOR UPDATE`)
- Consistent JSON error envelopes
- Swagger UI and OpenAPI 2.0 specs
- `golangci-lint` configuration with Git pre-commit hooks
- GitHub Actions CI (vet, lint, tests, build)

Repository: [https://github.com/iamadarsh0309/Trip-API](https://github.com/iamadarsh0309/Trip-API)

## Table of contents

- [Requirements](#requirements)
- [Quick start (Docker)](#quick-start-docker)
- [Local development](#local-development)
- [Environment variables](#environment-variables)
- [Make targets](#make-targets)
- [Linting](#linting)
- [Git pre-commit hooks](#git-pre-commit-hooks)
- [Continuous integration](#continuous-integration)
- [Tests](#tests)
- [API](#api)
- [Trip statuses](#trip-statuses)
- [Idempotency](#idempotency)
- [Migrations](#migrations)
- [Swagger / OpenAPI](#swagger--openapi)
- [Project layout](#project-layout)
- [Docker](#docker)
- [Troubleshooting](#troubleshooting)

## Requirements

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.23+ | `go.mod` declares `go 1.23.0`; Docker builds with `golang:1.23-alpine` |
| Docker + Docker Compose | current | Used for PostgreSQL and the full stack |
| Make | current | All common workflows are wrapped in `make` targets |
| golangci-lint | v2.x | Optional locally; `make lint` falls back to a pinned `go run` |
| PostgreSQL | 16 | Provided by `docker-compose.yml` (`postgres:16-alpine`) |

## Quick start (Docker)

```bash
docker compose up --build
```

- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger/index.html`
- PostgreSQL: `localhost:5432` (`trip` / `trip`, database `tripdb`)

The API container waits for PostgreSQL to be healthy, runs migrations on boot, then serves traffic.
Stop with `docker compose down`. Add `-v` to also delete the database volume.

## Local development

```bash
# 1. Start PostgreSQL only
docker compose up -d postgres

# 2. Run the API (make run supplies the default local DATABASE_URL)
make run

# Optional: run migrations explicitly (the API also migrates on boot)
make migrate-up
```

`make run` uses:

```text
DATABASE_URL=postgres://trip:trip@localhost:5432/tripdb?sslmode=disable
MIGRATIONS_PATH=file://migrations
```

To use a custom database, either override the variable or create a `.env` from
`.env.example` and export it:

```bash
cp .env.example .env
export $(grep -v '^#' .env | xargs)
make run
```

The process handles `SIGINT`/`SIGTERM` with a 10-second graceful shutdown.

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | yes | — | PostgreSQL connection string; `config.Load` fails if empty |
| `APP_PORT` | no | `8080` | HTTP listen port; must be numeric |
| `GIN_MODE` | no | `debug` | Gin mode (`debug`, `release`, `test`) |
| `MIGRATIONS_PATH` | no | `file://migrations` | `golang-migrate` source URL; Docker uses `file:///app/migrations` |

Environment is read directly from the process; no dotenv library is loaded.
The defaults above are used by `internal/config/config.go` and `docker-compose.yml`.

## Make targets

Run `make` or `make help` to list targets.

| Target | Command | Purpose |
|--------|---------|---------|
| `make tidy` | `go mod tidy` | Sync `go.mod` / `go.sum` |
| `make build` | `go build -o bin/trip-api ./cmd/api` | Build the API binary |
| `make run` | `go run ./cmd/api` | Run locally with the default DB URL |
| `make test` | `go test ./... -count=1` | Run all tests without the test cache |
| `make test-cover` | `go test ... -coverprofile=coverage.out` | Run tests and print per-function coverage |
| `make vet` | `go vet ./...` | Run Go's static checks |
| `make lint` | `golangci-lint run ./...` | Run the full linter suite |
| `make lint-fix` | `golangci-lint run --fix ./...` | Apply safe autofixes |
| `make fmt` | `gofmt -w .` + `golangci-lint fmt ./...` | Format code and imports |
| `make hooks` | `git config core.hooksPath .githooks` | Install the pre-commit hook |
| `make hooks-remove` | `git config --unset core.hooksPath` | Disable the hook |
| `make check` | `go vet` + lint + tests | Full local verification |
| `make swagger` | `swag init ...` | Regenerate `docs/` from handler annotations |
| `make migrate-up` | `migrate ... up` | Apply pending migrations |
| `make migrate-down` | `migrate ... down 1` | Roll back one migration |
| `make docker-up` | `docker compose up --build -d` | Start the full stack |
| `make docker-down` | `docker compose down` | Stop the full stack |
| `make clean` | `rm -rf bin coverage.out coverage.html` | Remove build/coverage artifacts |

Migration targets use `DATABASE_URL`; override it inline when needed:

```bash
make migrate-up DATABASE_URL='postgres://user:pass@localhost:5432/db?sslmode=disable'
```

## Linting

Linting is configured in [`.golangci.yml`](.golangci.yml) for **golangci-lint v2**.

```bash
make lint       # report issues
make lint-fix   # apply safe autofixes
make fmt        # gofmt + goimports
make vet        # go vet only
make check      # vet + lint + test
```

`make lint` uses an installed `golangci-lint` binary when available. If it is
not installed, it runs the pinned version (`v2.15.0`) through `go run`, so no
manual installation is required (the first run is slower while it compiles).
golangci-lint v2.15 requires Go 1.26 to build, so the fallback may download a
newer toolchain automatically (`GOTOOLCHAIN=auto`) even though the project
targets Go 1.23. Installing the binary avoids that download.

Install the binary for faster runs:

```bash
# macOS
brew install golangci-lint

# any platform with Go
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
```

Enabled configuration:

| Setting | Value |
|---------|-------|
| Default linters | `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused` |
| Extra linters | `bodyclose`, `errorlint`, `gocritic`, `misspell`, `nilerr`, `noctx`, `prealloc`, `unconvert`, `unparam`, `whitespace` |
| Formatters | `gofmt`, `goimports` |
| Generated code | Excluded (`generated: lax`), so `docs/docs.go` is not linted |
| Test exclusions | `bodyclose`, `noctx`, `unparam` are relaxed in `_test.go` files |
| Timeout | 5 minutes |

Formatting is part of `make lint`; a file that is not `gofmt`/`goimports` clean
will fail the lint step.

## Git pre-commit hooks

The repository ships a versioned hook at [`.githooks/pre-commit`](.githooks/pre-commit).
Install it once per clone:

```bash
make hooks
```

This runs `git config core.hooksPath .githooks` for the current repository.
When a commit contains staged `.go` files, the hook:

1. Verifies each staged file is `gofmt` clean.
2. Runs `make vet` over the module.
3. Runs `make lint` over the module.
4. Runs `make test` over the module.

Commits that stage no Go files skip the hook immediately. If a check fails, the
commit is blocked; fix the issue (`make fmt`, `make lint-fix`) and re-stage.

To bypass the hook for an exceptional commit:

```bash
git commit --no-verify -m "message"
```

To disable it permanently for the clone:

```bash
make hooks-remove
```

## Continuous integration

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on pushes to `main`
or `master` and on pull requests:

1. Install Go from `go.mod`.
2. Run golangci-lint `v2.15.0`.
3. `make vet`
4. `make test`
5. `make build`

## Tests

Tests are unit/integration-style tests with in-memory mock repositories; **no
PostgreSQL instance is required** to run them.

```bash
make test
make test-cover
```

Run a single test:

```bash
go test ./internal/service -run TestCreate_Success -count=1
go test ./internal/handler -run TestHTTP_CreateGetListUpdateDeleteFlow -count=1
```

Run a single package with verbose output:

```bash
go test ./internal/domain -v -count=1
```

Coverage includes domain status transitions, service idempotency/conflict
mapping, HTTP handler flows (create → replay → status → list → delete), config
loading, and error envelope helpers. `coverage.out` and `coverage.html` are
ignored by Git.

## API

Base URL (local): `http://localhost:8080`

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/trips` | Requires `Idempotency-Key` (16–128 chars) |
| `GET` | `/trips` | `status`, `page`, `page_size` query parameters |
| `GET` | `/trips/{id}` | UUID path parameter |
| `PATCH` | `/trips/{id}/status` | Valid status transitions only |
| `DELETE` | `/trips/{id}` | Permanent delete |
| `GET` | `/health` | Database ping |
| `GET` | `/swagger/index.html` | Interactive OpenAPI UI |

All API responses use `application/json` except `204 No Content` and the
Swagger UI. IDs are UUID v4
strings and timestamps are RFC3339 UTC. Every response carries `X-Request-ID`
(client value or generated UUID); request contexts time out after 15 seconds.

Quick create:

```bash
curl -s -X POST http://localhost:8080/trips \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: client-key-abc-00123' \
  -d '{
    "trip_number": "TRIP-1001",
    "source": "Bangalore",
    "destination": "Hyderabad",
    "driver_name": "Rahul Sharma"
  }'
```

Full endpoint reference (request/response examples, status codes, error codes,
curl cookbook):

- **[docs/API.md](docs/API.md)** — human-readable API docs
- **[docs/swagger.yaml](docs/swagger.yaml)** / **[docs/swagger.json](docs/swagger.json)** — OpenAPI 2.0 specs
- Interactive UI while the server runs: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

### Error envelope

All 4xx/5xx responses use:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "human readable summary",
    "details": ["optional field-level detail"]
  }
}
```

| Code | HTTP | When |
|------|------|------|
| `VALIDATION_ERROR` | 400 | Invalid body/query/path |
| `MISSING_IDEMPOTENCY_KEY` | 400 | `POST /trips` without the header |
| `BAD_REQUEST` | 400 | Malformed request |
| `NOT_FOUND` | 404 | Trip ID does not exist |
| `INVALID_STATUS_TRANSITION` | 409 | Illegal status change |
| `TRIP_NUMBER_CONFLICT` | 409 | Duplicate `trip_number` |
| `IDEMPOTENCY_KEY_REUSED` | 409 | Same key, different request body |
| `INTERNAL_ERROR` | 500 | Unexpected server/DB failure |

### Pagination and filtering

`GET /trips` supports:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | string | — | One of the five trip statuses |
| `page` | int | `1` | 1-based page index |
| `page_size` | int | `20` | Maximum `100` |

Results are ordered by `created_at` descending. Invalid statuses, pages, or page
sizes return `400 VALIDATION_ERROR`.

## Trip statuses

```text
CREATED → ASSIGNED → IN_TRANSIT → COMPLETED
```

`CANCELLED` is allowed from `CREATED`, `ASSIGNED`, or `IN_TRANSIT`.
`COMPLETED` and `CANCELLED` are terminal.

Rules enforced in `internal/domain/status.go` and mirrored by the database
`CHECK` constraint:

- Forward-only on the happy path; backwards moves and state skips are rejected.
- Same-status `PATCH` is a no-op returning `200`.
- Concurrent updates lock the row with `SELECT ... FOR UPDATE`; the `version`
  column increments on every real status change.

## Idempotency

`POST /trips` requires an `Idempotency-Key` header (16–128 characters).

- Same key + same request body → replayed `200` with the original response body.
- Same key + different body → `409 IDEMPOTENCY_KEY_REUSED`.
- `trip_number` is unique; duplicates → `409 TRIP_NUMBER_CONFLICT`.
- Trip creation and the cached idempotency response are written in one database
  transaction (`CreateWithIdempotency`).

Clients should reuse the same key for network retries, never for a new payload.

## Migrations

SQL migrations live in `migrations/` and are copied into the Docker image.

```text
migrations/
  000001_create_trips.up.sql / .down.sql
  000002_create_idempotency_keys.up.sql / .down.sql
```

- The API runs `migrate up` automatically on startup (`internal/database`).
- `make migrate-up` / `make migrate-down` are available for manual control.
- Add new migrations as paired, sequentially numbered `*.up.sql` and
  `*.down.sql` files. Keep the `trips.status` check constraint in sync with
  `internal/domain/status.go`.

## Swagger / OpenAPI

Swagger annotations live on handlers in `internal/handler/trip_handler.go` and
on `cmd/api/main.go`. Generated artifacts are committed under `docs/`.

```bash
make swagger
```

Note: the Make target uses `swag@latest`, so regenerating may upgrade the
generator and produce unrelated diffs. Review `docs/` changes before committing.
Do not hand-edit `docs/docs.go`; it is generated.

## Project layout

```text
cmd/api                 # entrypoint: config, migrations, DB pool, HTTP server
internal/config         # env config loading and validation
internal/database       # pgxpool setup and golang-migrate runner
internal/domain         # Trip model + status state machine
internal/dto            # request/response models and domain -> DTO mapping
internal/handler        # Gin HTTP layer, router, error response writing
internal/service        # business rules: idempotency, pagination, error mapping
internal/repository     # PostgreSQL access via pgx; idempotency transaction
internal/httperr        # AppError, error codes, JSON envelope
internal/middleware     # request ID and request timeout middleware
migrations/             # numbered up/down SQL migrations
docs/                   # generated Swagger docs + human API reference
.githooks/              # versioned Git hooks (pre-commit)
.golangci.yml           # golangci-lint v2 configuration
Dockerfile              # multi-stage build (Alpine runtime)
docker-compose.yml      # PostgreSQL + API stack
```

Request flow:

```text
HTTP -> middleware (request ID, timeout) -> handler -> service -> repository -> PostgreSQL
```

Handlers never return raw repository errors: the service maps repository
sentinels to `*httperr.AppError`, and `internal/handler/response.go` renders the
envelope.

## Docker

```bash
docker compose up --build      # foreground
make docker-up                 # detached + build
make docker-down               # stop
docker compose down -v         # stop and delete the database volume
docker compose logs -f api     # follow API logs
```

The `api` service:

- Builds from a multi-stage `Dockerfile` (`golang:1.23-alpine` → `alpine:3.20`).
- Copies `migrations/` into `/app/migrations`.
- Uses `GIN_MODE=release`, `APP_PORT=8080`, and
  `DATABASE_URL=postgres://trip:trip@postgres:5432/tripdb?sslmode=disable`.
- Waits for the PostgreSQL health check before starting.

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `DATABASE_URL is required` | Export `DATABASE_URL` or let `make run` supply the default |
| `APP_PORT must be a number` | Set `APP_PORT` to a numeric value |
| Port `5432` or `8080` already in use | Stop the conflicting service or change the published port |
| Migrations fail at startup | Check PostgreSQL is healthy and `MIGRATIONS_PATH` points at `migrations/` |
| `golangci-lint` not found | `make lint` uses a pinned `go run`; install it for faster runs (see [Linting](#linting)) |
| Pre-commit hook does not run | Run `make hooks`; verify with `git config --get core.hooksPath` |
| Swagger UI is empty after changes | Regenerate with `make swagger` and restart the server |
| Stale data after schema changes | `docker compose down -v` to recreate the volume (destroys data) |

## Additional documentation

- [docs/API.md](docs/API.md) — complete endpoint and curl reference
- [AGENTS.md](AGENTS.md) — repo-specific guidance for AI coding agents
