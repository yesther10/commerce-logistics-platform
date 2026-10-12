# Logistics Go Service

This service owns shipment records for the Commerce Logistics Platform. It stores a reference to `order_id`, but it does not model or write products, orders, customers, or payment data owned by another service.

## Why `net/http`

The first work unit is intentionally small and local. Go's standard `net/http` package provides the HTTP server, routing, request handling, and test integration needed here without introducing a web framework before the service boundaries are proven. The handler is kept separate from the domain and PostgreSQL repository so the HTTP behavior can be tested with deterministic in-memory fakes.

## Local setup

Start the service-owned PostgreSQL instance from the repository root:

```bash
cp .env.example .env
docker compose up -d postgres-logistics
```

Then configure and run the service:

```bash
cd services/logistics-go
cp .env.example .env
set -a
source .env
set +a
go run ./cmd/server
```

The server applies the SQL migrations in `internal/storage/migrations/` before listening. To run that step explicitly instead:

```bash
go run ./cmd/migrate
```

The migration runner records applied filenames in `schema_migrations` and applies each migration in a transaction. It is intentionally a small development path, not a replacement for production migration orchestration.

## HTTP API

```bash
curl http://127.0.0.1:8080/healthz

curl -i -X POST http://127.0.0.1:8080/shipments \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-123"}'

curl http://127.0.0.1:8080/shipments/<shipment_id>
```

`POST /shipments` validates a non-empty `order_id`, generates the shipment UUID in this service, stores the shipment with status `created`, and returns `201 Created`. `GET /shipments/{id}` returns `404 Not Found` when the shipment does not exist.

## RabbitMQ integration

The worker consumes `ShipmentRequested` from [`../../docs/events.md`](../../docs/events.md), creates the shipment and `ShipmentCreated` outbox record in one PostgreSQL transaction, and acknowledges only after that transaction commits. The consumer records `event_id` in `processed_events`, so duplicate deliveries are acknowledged without repeating business work.

The outbox publisher uses RabbitMQ publisher confirms and marks an event as published only after confirmation. A process failure between broker confirmation and the database update can publish the same `event_id` again; downstream consumers must therefore remain idempotent. Transient `ShipmentRequested` processing failures are published with the original body and identity to one of three durable retry queues (`1s`, `5s`, and `15s` by default), then acknowledged only after the retry publication is confirmed. Parsed permanent envelope or payload validation failures and exhausted retries are explicitly published to the final DLQ with bounded safe error metadata, then acknowledged. Malformed messages use the existing reject-to-DLQ path because they cannot be safely reconstructed. If retry or DLQ publication fails, or if the following acknowledgement fails, the worker stops without a successful acknowledgement so broker redelivery remains possible after recovery. A publication followed by acknowledgement failure may duplicate a retry or DLQ message; this is RabbitMQ at-least-once behavior, not exactly-once delivery.

### Run the HTTP server

```bash
go run ./cmd/server
```

The server keeps the existing `net/http` API and does not connect to RabbitMQ.

### Run the RabbitMQ worker

Start PostgreSQL and RabbitMQ from the repository root, copy `.env.example`, then set `RABBITMQ_ENABLED=true` in this service's environment:

```bash
go run ./cmd/worker
```

The worker applies migrations, declares the durable topic exchange `platform.events`, the durable DLX `platform.events.dlx`, its own durable queue and DLQ, and three new retry queues that dead-letter back to the main exchange after per-message expiration. It binds `commerce.shipment.requested`, uses manual acknowledgements, and sets prefetch to `1`. It also polls and publishes pending outbox events.

Important variables are `DATABASE_URL`, `RABBITMQ_ENABLED`, `RABBITMQ_URL`, `RABBITMQ_SHIPMENT_REQUESTED_QUEUE`, `RABBITMQ_SHIPMENT_REQUESTED_DLQ`, `RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_1`, `RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_2`, `RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_3`, `RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_1`, `RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_2`, `RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_3`, `RABBITMQ_CONFIRM_TIMEOUT`, `OUTBOX_POLL_INTERVAL`, and `OUTBOX_BATCH_SIZE`. Retry delays must be positive whole milliseconds and strictly increasing at the broker's millisecond granularity.

Worker attempt logs are JSON emitted with Go's standard `log/slog`. Each attempt includes service, component, queue, event identifiers, retry count, outcome, and a safe error class when relevant; event payloads and credentials are never logged.

## Tests and verification

Run these commands from this directory:

```bash
gofmt -w cmd internal
go test ./...
go vet ./...
git diff --check
```

The unit and HTTP tests use an in-memory fake repository and `httptest`; they do not require PostgreSQL or RabbitMQ. PostgreSQL is required for the service runtime, worker, and migration command. RabbitMQ is required only when the worker is enabled.

The Compose CI integration job runs `RABBITMQ_INTEGRATION=1 go test ./internal/messaging -run '^TestRabbitMQRetryTopologyIntegration$'` against a temporary RabbitMQ vhost. This check declares isolated queues, publishes three short-expiration retry messages, verifies that each expires back to the main queue, and verifies confirmed final-DLQ publication with the original identity and safe metadata. It proves broker topology, publication, routing, and TTL behavior without changing the full business flow. It does not inject processor failures into the running worker, so live failure classification and worker-driven exhaustion remain covered by unit tests rather than this broker check.

## Containerized processes

The root Compose `app` profile builds three reproducible targets from this service Dockerfile:

- `logistics-api` runs the existing `cmd/server` entrypoint and is published on host port `8080` by default.
- `logistics-worker` runs the existing `cmd/worker` entrypoint with RabbitMQ enabled. Its outbox publisher is the long-running polling loop already implemented by the worker; no shell loop is added.
- `logistics-migrator` runs the existing `cmd/migrate` entrypoint once after PostgreSQL is healthy.

The API and worker still apply the embedded migrations on startup because that is part of their real entrypoints. The one-shot migrator makes the migration boundary observable in the local Compose workflow; it is not a production migration orchestrator.
