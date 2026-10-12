# Local Docker environment

The Compose file has two deliberate modes:

- **Infrastructure-only (default):** RabbitMQ and the three service-owned PostgreSQL instances.
- **Full local stack (`app` profile):** the same dependencies plus the Laravel, Go, and NestJS application processes built from reproducible service Dockerfiles.

The application profile exists so the complete flow can run without PHP, Go, or Node.js installed on the host. It remains a development harness, not a production orchestrator.

## Quick path

From the repository root:

```bash
cp .env.example .env

# Validate either mode without starting containers.
docker compose --env-file .env.example config --quiet
docker compose --env-file .env.example --profile app config --quiet

# Infrastructure only: this remains the default behavior.
docker compose up -d
docker compose ps

# Full local stack.
docker compose --profile app up -d --build
docker compose --profile app ps
```

`docker compose up -d` starts only services without a profile. The `app` profile must be requested explicitly; this preserves the existing infrastructure-only workflow.

## Containers and processes

| Service | Profile | Responsibility | Host endpoint or state |
| --- | --- | --- | --- |
| `rabbitmq` | default | RabbitMQ broker and management UI | AMQP `127.0.0.1:5673`; UI `http://127.0.0.1:15673` |
| `postgres-commerce` | default | Commerce-owned PostgreSQL | `127.0.0.1:55432` |
| `postgres-logistics` | default | Logistics-owned PostgreSQL | `127.0.0.1:55433` |
| `postgres-notifications` | default | Notifications-owned PostgreSQL | `127.0.0.1:55434` |
| `commerce-api` | `app` | Laravel HTTP API and startup migrations | `http://127.0.0.1:8000`; health `/up` |
| `commerce-publisher` | `app` | Periodic Laravel outbox publisher | No host port; logs show publish attempts |
| `commerce-consumer` | `app` | Long-running `events:consume-order-created` consumer | No host port; logs show ACK/DLQ outcomes |
| `logistics-migrator` | `app` | One-shot `cmd/migrate` process | Exits successfully after migrations |
| `logistics-api` | `app` | Go `cmd/server` HTTP API | `http://127.0.0.1:8080`; health `/healthz` |
| `logistics-worker` | `app` | Go `cmd/worker` consumer and in-process outbox poller | No host port; logs show consumer/publisher activity |
| `notifications-api` | `app` | NestJS `dist/main.js` HTTP API | `http://127.0.0.1:3000`; health `/healthz` |
| `notifications-worker` | `app` | NestJS `dist/worker.js` consumers and outbox timer | No host port; logs show consumer/publisher activity |

The application processes use Compose DNS names, never `127.0.0.1`: PostgreSQL is reached as `postgres-commerce`, `postgres-logistics`, or `postgres-notifications`, and RabbitMQ as `rabbitmq`. Host ports are only for host-run clients and diagnostics.

If the default Go host port `8080` is already occupied, override only the published host port while keeping the container port unchanged:

```bash
LOGISTICS_API_PORT=19080 docker compose --profile app up -d
curl http://127.0.0.1:19080/healthz
```

The processes are intentionally separate:

- Commerce API, publisher, and `OrderCreated` consumer are three containers. The publisher uses the real `events:publish --loop=1` command, so it polls once per second without a shell loop.
- Logistics API and worker are separate. The worker already contains its long-running outbox polling loop; the migrator is a separate one-shot target.
- Notifications API and worker are separate. The API starts with RabbitMQ disabled; the worker enables both consumers and its existing outbox polling timer.

Outboxes and consumers can therefore be observed independently:

```bash
docker compose --profile app logs -f commerce-publisher commerce-consumer
docker compose --profile app logs -f logistics-worker notifications-worker
docker compose --profile app ps
```

## Healthchecks and dependencies

RabbitMQ uses `rabbitmq-diagnostics -q ping`; each PostgreSQL container uses `pg_isready`. The three APIs use their existing HTTP health endpoints. Workers wait for the relevant API/dependencies where that makes the database migration boundary safe; the Go migrator waits for healthy PostgreSQL and is a one-shot development job.

Inspect health directly:

```bash
docker compose ps
for service in rabbitmq postgres-commerce postgres-logistics postgres-notifications commerce-api logistics-api notifications-api; do
  container_id="$(docker compose ps -q "$service")"
  docker inspect --format '{{.Name}}: {{.State.Health.Status}}' "$container_id"
done
```

The migrator being `Exited (0)` is expected. A green healthcheck proves technical availability, not the complete event flow or idempotency behavior.

## Logs, rebuild, and shutdown

Follow one process or the complete stack:

```bash
docker compose logs -f commerce-api
docker compose --profile app logs -f
```

Rebuild application images after changing a Dockerfile, dependency lockfile, or application source:

```bash
docker compose --profile app build --no-cache
docker compose --profile app up -d
```

Stop containers while preserving named volumes:

```bash
docker compose down
```

Remove containers **and all local database/RabbitMQ data** only for an intentional reset:

```bash
docker compose down -v
```

Do not use `down -v` for routine restarts or verification. The named volumes are:

- `commerce-logistics-platform-rabbitmq-data`;
- `commerce-logistics-platform-commerce-postgres-data`;
- `commerce-logistics-platform-logistics-postgres-data`;
- `commerce-logistics-platform-notifications-postgres-data`.

## Environment and image inputs

Copy `.env.example` to `.env`. It contains development-only credentials, image tags, application ports, and the local Laravel key. `.env` is ignored by Git and must not be reused in production.

The service `.env.example` files describe host-run execution and therefore use `127.0.0.1` plus the published host ports. Compose overrides those values with internal DNS names and container ports through the service `environment` blocks. No `.env`, `vendor`, `node_modules`, local binaries, or secrets are copied into the application images.

## Network and local-development trade-offs

All containers use `platform-internal`. The network is intentionally not marked `internal: true`: on Linux, Docker internal-network mode prevented the published dependency ports from being reachable from the host even though Compose displayed the bindings. Removing that flag preserves container DNS and allows host diagnostics, but it is not a production isolation boundary.

## Limitations

This stack intentionally does not provide:

- production credentials, secret management, TLS, backups, or external provider integrations;
- an API Gateway, authentication, rate limiting, or centralized observability;
- production migration orchestration, rolling deployment, autoscaling, or resource policies;
- full RabbitMQ connection/channel recovery or replay automation; the logistics and notifications workers use bounded 1s/5s/15s delayed retry queues;
- real payment, carrier, email, SMS, or push delivery.

Consumers use additive durable topologies, manual acknowledgements, publisher confirms, local deduplication, bounded retries, and DLQs as documented in [`../../docs/events.md`](../../docs/events.md). Retry queues use TTL plus a DLX; malformed/permanent failures bypass retries, and retry/final-DLQ publication is confirmed before ACK. A confirmed publication followed by an ACK failure can redeliver the original message, so the flow remains at-least-once. Failed messages still require controlled DLQ inspection/replay. The full containerized smoke flow should be treated as local integration verification, not a production readiness signal.
