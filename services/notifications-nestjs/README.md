# Notifications NestJS Service

This service records notification requests in its own PostgreSQL database, exposes a small HTTP API, and integrates the notification segment of the RabbitMQ contract. Delivery remains simulated; no real email or SMS provider is connected.

## Quick path

From this directory:

```bash
npm install
cp .env.example .env
```

From the repository root, start the service-owned database:

```bash
cp .env.example .env # only if the root .env does not exist
docker compose up -d postgres-notifications
```

Then run the service:

```bash
npm run start:dev
```

The application runs on `http://127.0.0.1:3000` by default. Pending TypeORM migrations are applied at startup. RabbitMQ integration is disabled by default, so HTTP tests and local HTTP development do not need a broker. To run the migration explicitly instead:

```bash
npm run migration:run
```

## HTTP API

Health check:

```bash
curl http://127.0.0.1:3000/healthz
```

Create a local notification request:

```bash
curl -i -X POST http://127.0.0.1:3000/notifications \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-123","shipment_id":"shipment-123","notification_type":"shipment_created"}'
```

The response is `201 Created` and always starts with `status: "requested"`:

```json
{
  "id": "7b3f5a08-84fd-4576-a16b-e4e4ec2d8da9",
  "order_id": "order-123",
  "shipment_id": "shipment-123",
  "notification_type": "shipment_created",
  "status": "requested",
  "created_at": "2026-01-01T00:00:00.000Z",
  "updated_at": "2026-01-01T00:00:00.000Z"
}
```

Read a request by ID:

```bash
curl -i http://127.0.0.1:3000/notifications/<notification_id>
```

Invalid request bodies return `400 Bad Request`; an unknown valid UUID returns `404 Not Found`.

## Design

| Layer       | Responsibility                                               | Location                                        |
| ----------- | ------------------------------------------------------------ | ----------------------------------------------- |
| HTTP        | Routes, DTO validation, and HTTP errors                      | `src/health`, `src/notifications/http`          |
| Application | Use cases and the store abstraction                          | `src/notifications/application`                 |
| Persistence | TypeORM entity, PostgreSQL repository adapter, and migration | `src/notifications/persistence`, `src/database` |

NestJS modules compose these pieces. The controller depends on the application provider through NestJS dependency injection, while the application service depends on a small store contract rather than on HTTP or TypeORM details. Tests replace that store with an in-memory provider, so they are deterministic and do not require PostgreSQL.

TypeORM is configured for PostgreSQL through `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, and `DB_PASSWORD`. Schema synchronization is disabled (`synchronize: false`); the `notification_requests`, `processed_events`, and `outbox_events` tables are created only by versioned migrations. This service owns only notification request data and does not create order, shipment, or product tables.

## RabbitMQ integration

Set `RABBITMQ_ENABLED=true` to start the consumers and outbox publisher with the application:

```bash
npm run start:dev
```

For a worker-only process without the HTTP listener:

```bash
npm run build
npm run start:worker
```

The worker declares the durable topic exchange `platform.events`, the durable DLX `platform.events.dlx`, one durable queue and DLQ for `logistics.shipment.created`, and one durable queue and DLQ for `notifications.requested`. It adds the service-owned `platform.notifications.retry` exchange and three durable TTL queues with exactly 1,000 ms, 5,000 ms, and 15,000 ms delays. Retry messages use `CC` to preserve the original event routing key while the retry queue dead-letters back to `platform.events`; the existing main queue arguments are unchanged.

`ShipmentCreated` is deduplicated in `processed_events`; its local notification request and `NotificationRequested` outbox event are committed together. `NotificationRequested` is then deduplicated and transitions the local request from `requested` to `delivered` in a transaction. Delivery is simulated only.

The outbox publisher uses confirm channels and marks rows published only after broker confirmation. A crash after confirmation and before the database update can publish the same `event_id` again, which is why both consumers are idempotent. Malformed messages use broker `reject(message, false)` because their envelope cannot be safely reconstructed. Permanent validation/business failures go directly to the consumer-specific DLQ. Other processing failures receive at most three delayed retries; publication is confirmed before the original message is ACKed, so publication or ACK failure leaves redelivery possible and can create a duplicate.

Retry metadata is limited to `x-notifications-retry-count`, `x-notifications-retry-error-class`, and `x-notifications-retry-last-error`; logistics/commerce headers and arbitrary incoming headers are ignored. RabbitMQ variables are `RABBITMQ_ENABLED`, `RABBITMQ_URL`, the four existing queue/DLQ variables, `RABBITMQ_NOTIFICATIONS_RETRY_QUEUE_1..3`, `RABBITMQ_NOTIFICATIONS_RETRY_DELAY_1..3_MS`, `OUTBOX_POLL_INTERVAL`, and `OUTBOX_BATCH_SIZE`.

## Verification

Run the complete local checks:

```bash
npm run format
npm run format:check
npm run lint
npm test
npm run build
git diff --check
```

The unit and HTTP tests use mocked or in-memory providers. A running PostgreSQL instance is required only to boot the real application or run its migration command; RabbitMQ is required only when the integration is enabled. Run the broker-gated retry verification with `RABBITMQ_INTEGRATION=1 RABBITMQ_URL=... npm test -- --runInBand`; it waits for the real 1s/5s/15s TTL stages.

### Rollback

The retry topology is additive. To roll back this work, stop the notifications worker, remove the notifications retry queue/exchange declarations from the deployment configuration, and restore the worker revision. Do not delete the existing event queues or DLQs; their arguments are intentionally unchanged. Messages already in retry queues require controlled inspection before removal or replay.

## Intentionally deferred

The root Compose `app` profile runs `notifications-api` from `dist/main.js` with RabbitMQ disabled and `notifications-worker` from `dist/worker.js` with RabbitMQ enabled. The worker owns both long-lived consumers and the existing in-process outbox polling timer; no shell loop is added.

Real email/SMS providers, connection/channel recovery policy, an API Gateway, and production deployment remain outside this work unit. The retry topology is bounded and at-least-once: it reduces transient failure loss, but a confirmed retry followed by an ACK failure can produce a duplicate, which local inbox idempotency absorbs.
