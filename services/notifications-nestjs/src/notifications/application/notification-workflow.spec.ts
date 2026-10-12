import type { EventEnvelope } from '../../messaging/event-envelope';
import {
  NOTIFICATION_REQUESTED_EVENT,
  SHIPMENT_CREATED_EVENT,
} from '../../messaging/event-envelope';
import type {
  CreateNotificationRequest,
  NotificationRequestRecord,
} from './notification-request.store';
import {
  NotificationWorkflow,
  PermanentNotificationError,
  type NotificationEventTransaction,
  type NotificationEventUnitOfWork,
} from './notification-workflow';

describe('NotificationWorkflow', () => {
  it('creates one request and one outbox event for ShipmentCreated', async () => {
    const store = new FakeNotificationUnitOfWork();
    const ids = [
      '33333333-3333-3333-3333-333333333333',
      '44444444-4444-4444-4444-444444444444',
    ];
    const workflow = new NotificationWorkflow(
      () => ids.shift() as string,
      () => new Date('2026-01-01T12:00:00.000Z'),
    );
    const event = shipmentCreatedEvent();

    const first = await workflow.processShipmentCreated(store, event);
    const duplicate = await workflow.processShipmentCreated(store, event);

    expect(first.created).toBe(true);
    expect(first.event?.event_type).toBe(NOTIFICATION_REQUESTED_EVENT);
    expect(first.event?.causation_id).toBe(event.event_id);
    expect(duplicate.duplicate).toBe(true);
    expect(store.requests).toHaveLength(1);
    expect(store.outbox).toHaveLength(1);
  });

  it('transitions a requested notification to delivered idempotently', async () => {
    const store = new FakeNotificationUnitOfWork();
    const workflow = new NotificationWorkflow(
      () => '55555555-5555-5555-5555-555555555555',
      () => new Date('2026-01-01T12:00:00.000Z'),
    );
    await workflow.processShipmentCreated(store, shipmentCreatedEvent());
    const notificationEvent = store.outbox[0];

    const delivered = await workflow.processNotificationRequested(
      store,
      notificationEvent,
    );
    const duplicate = await workflow.processNotificationRequested(
      store,
      notificationEvent,
    );

    expect(delivered.delivered).toBe(true);
    expect(delivered.request?.status).toBe('delivered');
    expect(duplicate.duplicate).toBe(true);
    expect(store.requests[0].status).toBe('delivered');
  });

  it('classifies invalid event contracts as permanent notification failures', async () => {
    const workflow = new NotificationWorkflow(
      () => '55555555-5555-5555-5555-555555555555',
      () => new Date('2026-01-01T12:00:00.000Z'),
    );
    const event = shipmentCreatedEvent();
    event.event_version = 2;

    expect(() =>
      workflow.processShipmentCreated(new FakeNotificationUnitOfWork(), event),
    ).toThrow(PermanentNotificationError);
  });
});

function shipmentCreatedEvent(): EventEnvelope {
  return {
    event_id: '11111111-1111-1111-1111-111111111111',
    event_type: SHIPMENT_CREATED_EVENT,
    event_version: 1,
    occurred_at: '2026-01-01T12:00:00Z',
    correlation_id: '22222222-2222-2222-2222-222222222222',
    causation_id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    payload: {
      order_id: 'order-1',
      shipment_id: 'shipment-1',
      shipment_status: 'created',
    },
  };
}

class FakeNotificationUnitOfWork implements NotificationEventUnitOfWork {
  processed = new Set<string>();
  requests: NotificationRequestRecord[] = [];
  outbox: EventEnvelope[] = [];

  withTransaction<T>(
    work: (transaction: NotificationEventTransaction) => Promise<T>,
  ): Promise<T> {
    return work({
      markEventProcessed: async (event) => {
        if (this.processed.has(event.event_id)) return false;
        this.processed.add(event.event_id);
        return true;
      },
      findByIdentity: async (input) =>
        this.requests.find(
          (request) =>
            request.orderId === input.orderId &&
            request.shipmentId === input.shipmentId &&
            request.notificationType === input.notificationType,
        ) ?? null,
      create: async (input: CreateNotificationRequest & { id: string }) => {
        const now = new Date('2026-01-01T12:00:00.000Z');
        const request: NotificationRequestRecord = {
          ...input,
          status: 'requested',
          createdAt: now,
          updatedAt: now,
        };
        this.requests.push(request);
        return request;
      },
      addOutboxEvent: async (event) => {
        this.outbox.push(event);
      },
      markDelivered: async (input) => {
        const request = this.requests.find(
          (candidate) =>
            candidate.orderId === input.orderId &&
            candidate.shipmentId === input.shipmentId &&
            candidate.notificationType === input.notificationType &&
            candidate.status === 'requested',
        );
        if (!request) return false;
        request.status = 'delivered';
        return true;
      },
    });
  }
}
