# Trip Management API

Production-oriented Go (Gin) REST service for managing trips with:

- PostgreSQL persistence and schema migrations
- Idempotent `POST /trips` via `Idempotency-Key`
- Enforced status transitions
- Safe concurrent status updates (`SELECT ... FOR UPDATE`)
- Swagger UI

Repository: [https://github.com/iamadarsh0309/Trip-API](https://github.com/iamadarsh0309/Trip-API)

## Trip statuses

```
CREATED → ASSIGNED → IN_TRANSIT → COMPLETED
```

`CANCELLED` is allowed from `CREATED`, `ASSIGNED`, or `IN_TRANSIT`.  
`COMPLETED` and `CANCELLED` are terminal.

## Quick start (Docker)

```bash
docker compose up --build
```

API: `http://localhost:8080`  
Swagger: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

## Local development

```bash
# start Postgres only
docker compose up -d postgres

cp .env.example .env
export $(grep -v '^#' .env | xargs)

make migrate-up   # optional; API also migrates on boot
make run
```

## API overview

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/trips` | Requires `Idempotency-Key` header |
| `GET` | `/trips` | `status`, `page`, `page_size` |
| `GET` | `/trips/{id}` | |
| `PATCH` | `/trips/{id}/status` | Valid transitions only |
| `DELETE` | `/trips/{id}` | |
| `GET` | `/health` | DB ping |
| `GET` | `/swagger/index.html` | OpenAPI UI |

### Create a trip

```bash
curl -s -X POST http://localhost:8080/trips \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: client-key-abc-001' \
  -d '{
    "trip_number": "TRIP-1001",
    "source": "Bangalore",
    "destination": "Hyderabad",
    "driver_name": "Rahul Sharma"
  }'
```

Replaying the same key + body returns **200** with the original trip payload.  
Reusing the key with a different body returns **409** `IDEMPOTENCY_KEY_REUSED`.

### Update status

```bash
curl -s -X PATCH http://localhost:8080/trips/<TRIP_ID>/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"ASSIGNED"}'
```

### List with filter + pagination

```bash
curl -s 'http://localhost:8080/trips?status=CREATED&page=1&page_size=20'
```

## Error envelope

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "invalid request",
    "details": []
  }
}
```

Common codes: `VALIDATION_ERROR`, `NOT_FOUND`, `INVALID_STATUS_TRANSITION`, `TRIP_NUMBER_CONFLICT`, `IDEMPOTENCY_KEY_REUSED`, `MISSING_IDEMPOTENCY_KEY`.

## Project layout

```
cmd/api                 # entrypoint
internal/config         # env config
internal/domain         # trip + status machine
internal/dto            # request/response models
internal/handler        # Gin HTTP layer
internal/service        # business rules
internal/repository     # PostgreSQL (pgx)
internal/httperr        # consistent errors
migrations/             # SQL migrations
docs/                   # generated Swagger
```

## Make targets

```bash
make tidy
make test
make swagger
make run
make docker-up
make docker-down
```

## Tests

```bash
make test
```
