# Initial Architecture

This document fixes the first implementation design for Commerce Logistics Platform. It keeps the three services independently owned while using RabbitMQ for the order-to-shipment lifecycle. The design is intentionally small: it establishes boundaries and contracts without adding application, infrastructure, or integration features that have not been agreed.

## Design at a glance

| Area | Initial decision |
| --- | --- |
| Service boundaries | `commerce-laravel`, `notifications-nestjs`, and `logistics-go` remain separate deployable services. |
| Data ownership | Each service owns its domain data and its own PostgreSQL database. No shared database or cross-service writes. |
| Lifecycle integration | RabbitMQ events carry the first-stage order-to-shipment workflow. |
| Client access | Clients address the service that owns the requested capability. There is no API Gateway in the first stage. |
| Synchronous communication | Local commands and direct client request/response interactions only. The initial lifecycle does not require service-to-service HTTP calls. |
| Asynchronous communication | Domain transitions between service boundaries use the event contract in [`events.md`](events.md). |
| Payment and delivery | Payment approval and notification delivery are simulated. No external provider is integrated. |

## Service responsibilities and ownership

The following responsibilities are definitive for the first implementation stage. They may be revisited by a later architectural decision, but no service should assume ownership outside this table.

### `commerce-laravel`

**Owns:**

- Products.
- Customers.
- Carts.
- Orders and order lifecycle state.
- The first-stage payment approval state, including the simulated payment transition.

**Publishes:** `OrderCreated`, `PaymentApproved`, and `ShipmentRequested`.

**Does not own:** shipments, delivery tracking, or notification delivery records.

The payment step is a commerce-owned simulation. There is no payment service and no external payment provider in this stage. The simulation exists only to make the lifecycle contract observable.

### `logistics-go`

**Owns:**

- Shipments.
- Shipment planning state required by the first-stage flow.
- Delivery tracking and logistics status.

**Consumes:** `ShipmentRequested`.

**Publishes:** `ShipmentCreated`.

`logistics-go` stores references such as `order_id`; it does not become the owner of order, customer, product, or payment data.

### `notifications-nestjs`

**Owns:**

- Notification requests and delivery state needed by the first-stage flow.
- The local record of simulated notification handling.

**Consumes:** `ShipmentCreated` and `NotificationRequested`.

After consuming `ShipmentCreated`, the service publishes `NotificationRequested` for its own delivery workflow. A notification delivery handler then consumes that request.

Notification delivery is simulated. The service does not own customer, order, shipment, or payment data and must not update those domains directly.

## Data boundaries

- Each service has an independent PostgreSQL database.
- A service may store identifiers and the minimum event data needed for its own processing, but it does not replicate another service's full aggregate.
- No service reads another service's database or performs cross-database joins.
- Cross-service state changes are requested through versioned RabbitMQ events, not through direct database access.
- Event payloads are not a substitute for ownership. A copied `order_id` in logistics is a reference, not an order record owned by logistics.

## Communication rules

### Synchronous communication

Synchronous request/response is limited to a client calling the service that owns the requested capability and to work performed inside one service transaction. The first-stage lifecycle does not depend on a synchronous HTTP call from one service to another.

This keeps a service from blocking order processing on the availability of logistics or notifications. It also avoids creating an unplanned cross-service API contract before the domain boundaries are exercised.

### Asynchronous communication

RabbitMQ is the integration boundary for the lifecycle:

```text
commerce-laravel
  OrderCreated -> PaymentApproved -> ShipmentRequested
                                      |
                                      v
                               logistics-go
                         ShipmentCreated
                              |
                              v
                  notifications-nestjs
                     NotificationRequested
```

The exact event envelope, payloads, correlation rules, retries, and dead-letter behavior are defined in [`events.md`](events.md).

## First-stage happy path

1. `commerce-laravel` creates an order and publishes `OrderCreated`.
2. A commerce-owned payment simulator approves the order and publishes `PaymentApproved`.
3. Commerce publishes `ShipmentRequested` for the approved order.
4. `logistics-go` creates a shipment record and publishes `ShipmentCreated`.
5. `notifications-nestjs` consumes `ShipmentCreated` and publishes `NotificationRequested`.
6. `notifications-nestjs` records and simulates handling of the notification request.

The workflow is intentionally not a distributed transaction. Each consumer commits its own state and acknowledges the message only after its local work succeeds. Duplicate delivery is expected and handled through idempotent consumers.

## First-stage limits

### Included

- The three service boundaries and their data ownership.
- Independent PostgreSQL databases, one per service.
- RabbitMQ lifecycle events and the operational rules in `events.md`.
- A simulated payment approval transition.
- Shipment creation as the first logistics result.
- A simulated notification request and delivery step.

### Explicitly excluded

- An API Gateway.
- A real payment provider or payment service.
- A real carrier, shipping-rate provider, or route-optimization integration.
- A notification provider integration.
- Shared databases, shared domain libraries, or synchronous service-to-service HTTP for the lifecycle.
- Real notification providers, full RabbitMQ connection recovery, an API Gateway, and production deployment orchestration.

## Decision: no API Gateway initially

The first stage does not introduce an API Gateway. Clients use the service boundary that owns the capability they need, and the lifecycle between services uses RabbitMQ directly.

### Why this is the right first trade-off

- The project has only three services and no agreed gateway requirements.
- A gateway would add routing, aggregation, authentication propagation, deployment, and observability concerns before those concerns are needed.
- Direct access makes service ownership visible while the boundaries are still being learned.
- Removing one infrastructure component keeps the educational setup and failure modes easier to understand.

### Accepted costs

- Clients must know which service owns a capability.
- Cross-service views are not aggregated at the edge in this stage.
- Cross-cutting concerns such as authentication, rate limiting, and edge observability are not centralized yet.

These costs are accepted temporarily, not ignored. A future gateway requires a separate decision based on demonstrated needs such as a stable public API, edge policy centralization, or response aggregation. It must not be added merely to hide unclear service ownership.

## Decisions that must not be inferred

- `PaymentApproved` does not mean a real payment was authorized by an external system.
- `ShipmentCreated` does not imply carrier booking, routing, or delivery completion.
- `NotificationRequested` does not imply that a message was delivered to a real channel.
- Eventual consistency is expected between service databases; there is no cross-service transaction in this stage.
