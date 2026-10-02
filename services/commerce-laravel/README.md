# Commerce Laravel Service

This service owns the commerce domain for the first work unit: products, orders, order items, the simulated payment transition, and the order total calculated from server-side product prices. It does not own shipments or notifications.

## Runtime

Laravel 13 requires PHP 8.3 or newer. The host PHP and Composer are not required: the application was bootstrapped and can be verified with the PHP 8.5 Composer development image.

Install the repository infrastructure first from the repository root:

```bash
cp .env.example .env
docker compose up -d postgres-commerce
```

The service `.env.example` is configured for host execution through the published PostgreSQL port (`127.0.0.1:55432`). Copy it before running the service locally:

```bash
cp services/commerce-laravel/.env.example services/commerce-laravel/.env
```

Run Artisan commands in Docker from the repository root:

```bash
docker run --rm \
  -u "$(id -u):$(id -g)" \
  --network host \
  -v "$PWD/services/commerce-laravel:/app" \
  -w /app \
  --entrypoint php composer:2 artisan migrate
```

The `--network host` option is for Linux and lets the development container use the default PostgreSQL host settings. On Docker Desktop, omit that option and override `DB_HOST` with `host.docker.internal` (using a host-gateway mapping when required). Host execution is supported only with PHP 8.3+ and Composer; the current host PHP 8.2 runtime is intentionally not used.

Run the automated suite with the same runtime image. Tests use in-memory SQLite through `phpunit.xml`, so they do not require PostgreSQL:

```bash
docker run --rm \
  -u "$(id -u):$(id -g)" \
  -v "$PWD/services/commerce-laravel:/app" \
  -w /app \
  --entrypoint php composer:2 artisan test
```

## API

The initial API is intentionally public and unversioned authentication is out of scope:

- `POST /api/v1/products` — create a product with `name` and `price`.
- `GET /api/v1/products` — list products.
- `POST /api/v1/orders` — create an order from product IDs and quantities.
- `GET /api/v1/orders/{order}` — retrieve an order with its items and product details.

Order creation validates the request, locks the selected product rows, calculates line totals and the order total in integer cents, and persists the order and items in one local database transaction. Product prices are never accepted from the client.

## Events and RabbitMQ

The service implements the first commerce segment from [`../../docs/events.md`](../../docs/events.md):

- Order creation writes an `OrderCreated` envelope to `outbox_events` in the same local transaction as the order.
- `PaymentApproved` and `ShipmentRequested` are written to the same outbox when the commerce consumer processes `OrderCreated`.
- The envelope contains `event_id`, `event_type`, `event_version`, `occurred_at`, `correlation_id`, `causation_id`, and `payload`.
- The publisher declares the durable topic exchange `platform.events` and waits for RabbitMQ publisher confirmation before marking an outbox row as published.
- If the process stops after RabbitMQ confirms a message and before the database update, the event can be published again. This is intentionally at-least-once delivery; every consumer must deduplicate by `event_id` and make its business transition idempotent.
- The consumer owns the durable `commerce.order.created` queue, binds `commerce.order.created`, acknowledges manually after the database transaction commits, and records processed event IDs in `processed_events`. Invalid or failed messages are rejected to the commerce dead-letter queue.

RabbitMQ settings are read from `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASSWORD`, and the related timeout/queue variables in `.env.example`. With the repository infrastructure running, host execution uses `127.0.0.1:5673`.

Publish pending events from the service directory:

```bash
php artisan events:publish
php artisan events:publish --limit=50
```

Run the commerce payment consumer continuously, or process one message for a smoke test:

```bash
php artisan events:consume-order-created
php artisan events:consume-order-created --once
```

These commands require a PHP runtime with the `sockets` extension because `php-amqplib/php-amqplib` uses AMQP network sockets. The automated test suite uses mocks and SQLite and does not require RabbitMQ.

The root Compose file can run this service without host PHP through the `app` profile. `commerce-api` serves HTTP, `commerce-publisher` runs the periodic outbox publisher, and `commerce-consumer` runs the long-lived `events:consume-order-created` process. The publisher uses the existing command with `--loop=1`, which polls once per second and keeps broker failures visible in its logs.

Authentication, real payments, an API Gateway, shipment or notification models, and production deployment remain out of scope.
