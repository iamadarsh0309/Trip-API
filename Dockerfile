# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/trip-api ./cmd/api

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/trip-api /app/trip-api
COPY migrations /app/migrations

ENV APP_PORT=8080
ENV GIN_MODE=release

EXPOSE 8080

ENTRYPOINT ["/app/trip-api"]
