# Trip Management API Documentation

Base URL (local): `http://localhost:8080`

Interactive OpenAPI UI: [`/swagger/index.html`](http://localhost:8080/swagger/index.html)

Machine-readable specs:

- [`swagger.yaml`](./swagger.yaml)
- [`swagger.json`](./swagger.json)

---

## Conventions

| Item | Detail |
|------|--------|
| Content type | `application/json` |
| IDs | UUID v4 strings |
| Timestamps | RFC3339 UTC (`created_at`, `updated_at`) |
| Pagination | `page` (default `1`), `page_size` (default `20`, max `100`) |
| Idempotency | Required on `POST /trips` via `Idempotency-Key` header (16–128 chars) |

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
| `MISSING_IDEMPOTENCY_KEY` | 400 | `POST /trips` without header |
| `NOT_FOUND` | 404 | Trip ID does not exist |
| `INVALID_STATUS_TRANSITION` | 409 | Illegal status change |
| `TRIP_NUMBER_CONFLICT` | 409 | Duplicate `trip_number` |
| `IDEMPOTENCY_KEY_REUSED` | 409 | Same key, different body |
| `INTERNAL_ERROR` | 500 | Unexpected server/DB failure |

---

## Status lifecycle

```
CREATED ──► ASSIGNED ──► IN_TRANSIT ──► COMPLETED
   │            │              │
   └────────────┴──────────────┴──► CANCELLED
```

Rules:

- Forward-only on the happy path (no backwards moves).
- Skipping states is rejected (e.g. `CREATED → IN_TRANSIT`).
- `COMPLETED` and `CANCELLED` are terminal.
- Same-status PATCH is a no-op and returns **200**.

---

## Endpoints

### `GET /health`

Liveness/readiness check (DB ping).

**Response `200`**

```json
{ "status": "ok" }
```

**Response `503`** — database unavailable.

---

### `POST /trips`

Create a trip. Initial status is always `CREATED`.

**Headers**

| Header | Required | Description |
|--------|----------|-------------|
| `Idempotency-Key` | yes | Client-generated key, 16–128 characters |
| `Content-Type` | yes | `application/json` |

**Request body**

| Field | Type | Rules |
|-------|------|-------|
| `trip_number` | string | required, unique, 1–64 chars |
| `source` | string | required, 1–255 chars |
| `destination` | string | required, 1–255 chars |
| `driver_name` | string | required, 1–255 chars |

**Example**

```http
POST /trips HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Idempotency-Key: client-key-abc-00123

{
  "trip_number": "TRIP-1001",
  "source": "Bangalore",
  "destination": "Hyderabad",
  "driver_name": "Rahul Sharma"
}
```

**Response `201 Created`**

```json
{
  "id": "f343db89-7838-45d8-9e27-e3f10ac11b7e",
  "trip_number": "TRIP-1001",
  "source": "Bangalore",
  "destination": "Hyderabad",
  "driver_name": "Rahul Sharma",
  "status": "CREATED",
  "created_at": "2026-10-27T06:38:11.267171Z",
  "updated_at": "2026-10-27T06:38:11.267171Z"
}
```

**Response `200 OK`** — idempotent replay of the same key + same body (returns original trip payload).

**Errors**

| Status | Code | Cause |
|--------|------|-------|
| 400 | `MISSING_IDEMPOTENCY_KEY` | Header missing |
| 400 | `VALIDATION_ERROR` | Bad/missing fields or short key |
| 409 | `TRIP_NUMBER_CONFLICT` | `trip_number` already exists |
| 409 | `IDEMPOTENCY_KEY_REUSED` | Key reused with a different body |

---

### `GET /trips`

List trips with optional status filter and pagination. Ordered by `created_at` descending.

**Query parameters**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | string | — | One of `CREATED`, `ASSIGNED`, `IN_TRANSIT`, `COMPLETED`, `CANCELLED` |
| `page` | int | `1` | 1-based page index |
| `page_size` | int | `20` | Max `100` |

**Example**

```http
GET /trips?status=ASSIGNED&page=1&page_size=10 HTTP/1.1
Host: localhost:8080
```

**Response `200`**

```json
{
  "items": [
    {
      "id": "f343db89-7838-45d8-9e27-e3f10ac11b7e",
      "trip_number": "TRIP-1001",
      "source": "Bangalore",
      "destination": "Hyderabad",
      "driver_name": "Rahul Sharma",
      "status": "ASSIGNED",
      "created_at": "2026-10-27T06:38:11.267171Z",
      "updated_at": "2026-10-27T06:38:11.355861Z"
    }
  ],
  "page": 1,
  "page_size": 10,
  "total": 1
}
```

---

### `GET /trips/{id}`

Fetch a single trip by UUID.

**Response `200`** — trip object (same shape as create response).

**Response `404`**

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "trip not found",
    "details": []
  }
}
```

---

### `PATCH /trips/{id}/status`

Update trip status. Concurrent updates are serialized with `SELECT ... FOR UPDATE`.

**Request body**

```json
{ "status": "ASSIGNED" }
```

**Example sequence**

```http
PATCH /trips/f343db89-7838-45d8-9e27-e3f10ac11b7e/status
{ "status": "ASSIGNED" }

PATCH /trips/f343db89-7838-45d8-9e27-e3f10ac11b7e/status
{ "status": "IN_TRANSIT" }

PATCH /trips/f343db89-7838-45d8-9e27-e3f10ac11b7e/status
{ "status": "COMPLETED" }
```

**Response `200`** — updated trip.

**Response `409` (invalid transition)**

```json
{
  "error": {
    "code": "INVALID_STATUS_TRANSITION",
    "message": "invalid status transition from CREATED to COMPLETED",
    "details": ["from=CREATED", "to=COMPLETED"]
  }
}
```

---

### `DELETE /trips/{id}`

Permanently deletes a trip.

**Response `204 No Content`** — success (empty body).

**Response `404`** — trip not found.

---

## curl cookbook

```bash
# Health
curl -s http://localhost:8080/health

# Create
curl -s -X POST http://localhost:8080/trips \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: client-key-abc-00123' \
  -d '{"trip_number":"TRIP-1001","source":"Bangalore","destination":"Hyderabad","driver_name":"Rahul Sharma"}'

# Idempotent replay (expect 200)
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/trips \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: client-key-abc-00123' \
  -d '{"trip_number":"TRIP-1001","source":"Bangalore","destination":"Hyderabad","driver_name":"Rahul Sharma"}'

# Get / list / status / delete
curl -s http://localhost:8080/trips/$ID
curl -s 'http://localhost:8080/trips?status=CREATED&page=1&page_size=20'
curl -s -X PATCH http://localhost:8080/trips/$ID/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"ASSIGNED"}'
curl -s -o /dev/null -w "%{http_code}\n" -X DELETE http://localhost:8080/trips/$ID
```

---

## Concurrency & idempotency notes

1. **Status updates** — two concurrent `PATCH` requests lock the same row; the second applies only if the transition is still valid after the first commit.
2. **Create retries** — clients should reuse the same `Idempotency-Key` for network retries so duplicate trips are not inserted.
3. **Trip numbers** — uniqueness is enforced by a database unique constraint in addition to application checks.
