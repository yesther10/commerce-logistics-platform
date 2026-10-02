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

The worker declares the durable topic exchange `platform.events`, the durable DLX `platform.events.dlx`, one durable queue and DLQ for `logistics.shipment.created`, and one durable queue and DLQ for `notifications.requested`. It binds the exact routing keys from [`../../docs/events.md`](../../docs/events.md), uses manual acknowledgements, and sets prefetch to `1`.

`ShipmentCreated` is deduplicated in `processed_events`; its local notification request and `NotificationRequested` outbox event are committed together. `NotificationRequested` is then deduplicated and transitions the local request from `requested` to `delivered` in a transaction. Delivery is simulated only.

The outbox publisher uses confirm channels and marks rows published only after broker confirmation. A crash after confirmation and before the database update can publish the same `event_id` again, which is why both consumers are idempotent. Delayed retries are not implemented yet; failures are rejected to the consumer-specific DLQ through `platform.events.dlx` for controlled inspection or replay.

RabbitMQ variables are `RABBITMQ_ENABLED`, `RABBITMQ_URL`, `RABBITMQ_SHIPMENT_CREATED_QUEUE`, `RABBITMQ_SHIPMENT_CREATED_DLQ`, `RABBITMQ_NOTIFICATION_REQUESTED_QUEUE`, `RABBITMQ_NOTIFICATION_REQUESTED_DLQ`, `OUTBOX_POLL_INTERVAL`, and `OUTBOX_BATCH_SIZE`.

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

The unit and HTTP tests use mocked or in-memory providers. A running PostgreSQL instance is required only to boot the real application or run its migration command; RabbitMQ is required only when the integration is enabled.

## Intentionally deferred

The root Compose `app` profile runs `notifications-api` from `dist/main.js` with RabbitMQ disabled and `notifications-worker` from `dist/worker.js` with RabbitMQ enabled. The worker owns both long-lived consumers and the existing in-process outbox polling timer; no shell loop is added.

Real email/SMS providers, delayed automatic retries, connection recovery policy, an API Gateway, and production deployment remain outside this work unit.
