# Commerce Logistics Platform

Commerce Logistics Platform is an educational microservices project for exploring an e-commerce domain with integrated logistics capabilities.

## Scope

The agreed initial scope is an e-commerce platform with logistics integrated into the order lifecycle. The repository contains the three service boundaries, local Docker infrastructure, and the first local application and RabbitMQ integration work units.

The first stage will not include an API Gateway. Clients and service interactions will use the service boundaries directly until a gateway is justified by a later architectural decision.

## Provisional service responsibilities

| Service | Responsibility | Planned technology |
| --- | --- | --- |
| `commerce-laravel` | Product, cart, customer, and order management | Laravel / PHP |
| `notifications-nestjs` | Notification orchestration and delivery events | NestJS / Node.js |
| `logistics-go` | Shipment planning, delivery tracking, and logistics status | Go |

These responsibilities are provisional and may evolve as the educational implementation progresses.

## Infrastructure direction

- **RabbitMQ** will provide asynchronous messaging between services.
- **PostgreSQL** will be provisioned independently for each service to preserve service data ownership.
- **Docker Compose** provisions the local RabbitMQ broker, one PostgreSQL instance per service, and an optional containerized application profile. See [`infrastructure/docker/README.md`](infrastructure/docker/README.md).

The default Compose workflow remains infrastructure-only. The `app` profile builds and runs the Laravel API/publisher/consumer, Go API/worker/migrator, and NestJS API/worker without requiring PHP, Go, or Node.js on the host. Application services remain independently runnable as well.

```bash
cp .env.example .env
docker compose up -d
docker compose --profile app up -d --build
```

The containerized stack is a local development harness, not a production deployment: it uses development credentials, direct service ports, simulated payment/notification behavior, and no TLS, authentication, automatic RabbitMQ reconnection, delayed retries, or production orchestration.

## Repository layout

```text
services/
├── commerce-laravel/
├── notifications-nestjs/
└── logistics-go/
infrastructure/
└── docker/
    └── README.md
docker-compose.yml
docs/
```

## Status

Initial service implementations plus local Docker infrastructure and an optional containerized application profile. The current integration completes the `ShipmentRequested` -> `ShipmentCreated` -> `NotificationRequested` -> simulated delivery path with local outboxes, inbox deduplication, publisher confirms, and durable DLQs.

## Educational documentation

For a from-scratch explanation of the domain, service boundaries, event contract, reliability decisions, Docker setup, verification strategy, and recommended learning path, read the [Decisions and Learning Guide](docs/decisions-and-learning-guide.md).

## Contribution and delivery workflow

- [Contribution guide](CONTRIBUTING.md) — branches, work units, commits, reviews, releases, and local checks.
- [Git workflow guide](docs/git-workflow.md) — an educational explanation of the repository's Git Flow variant and feature-branch chain.
