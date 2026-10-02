import type { EventEnvelope } from '../../messaging/event-envelope';
import {
  newEventEnvelope,
  NOTIFICATION_REQUESTED_EVENT,
  requireEventType,
  SHIPMENT_CREATED_EVENT,
} from '../../messaging/event-envelope';
import type {
  CreateNotificationRequest,
  NotificationRequestRecord,
} from './notification-request.store';

export const NOTIFICATION_EVENT_UNIT_OF_WORK = Symbol(
  'NOTIFICATION_EVENT_UNIT_OF_WORK',
);

export interface NotificationEventTransaction {
  markEventProcessed(event: EventEnvelope, processedAt: Date): Promise<boolean>;
  findByIdentity(
    input: CreateNotificationRequest,
  ): Promise<NotificationRequestRecord | null>;
  create(
    input: CreateNotificationRequest & { id: string },
  ): Promise<NotificationRequestRecord>;
  addOutboxEvent(event: EventEnvelope): Promise<void>;
  markDelivered(input: CreateNotificationRequest): Promise<boolean>;
}

export interface NotificationEventUnitOfWork {
  withTransaction<T>(
    work: (transaction: NotificationEventTransaction) => Promise<T>,
  ): Promise<T>;
}

export interface NotificationWorkflowResult {
  duplicate: boolean;
  created: boolean;
  delivered: boolean;
  request: NotificationRequestRecord | null;
  event: EventEnvelope | null;
}

export class NotificationWorkflow {
  constructor(
    private readonly newId: () => string,
    private readonly now: () => Date,
  ) {}

  processShipmentCreated(
    unitOfWork: NotificationEventUnitOfWork,
    event: EventEnvelope,
  ): Promise<NotificationWorkflowResult> {
    requireEventType(event, SHIPMENT_CREATED_EVENT);
    const payload = shipmentCreatedPayload(event);
    const input = {
      orderId: payload.order_id,
      shipmentId: payload.shipment_id,
      notificationType: 'shipment_created',
    };

    return unitOfWork.withTransaction(async (transaction) => {
      const inserted = await transaction.markEventProcessed(event, this.now());
      if (!inserted) {
        return {
          duplicate: true,
          created: false,
          delivered: false,
          request: null,
          event: null,
        };
      }
      const existing = await transaction.findByIdentity(input);
      if (existing) {
        return {
          duplicate: false,
          created: false,
          delivered: false,
          request: existing,
          event: null,
        };
      }
      const request = await transaction.create({ ...input, id: this.newId() });
      const notificationEvent = newEventEnvelope(
        NOTIFICATION_REQUESTED_EVENT,
        event.correlation_id,
        event.event_id,
        inputToPayload(input),
        this.now(),
        this.newId(),
      );
      await transaction.addOutboxEvent(notificationEvent);
      return {
        duplicate: false,
        created: true,
        delivered: false,
        request,
        event: notificationEvent,
      };
    });
  }

  processNotificationRequested(
    unitOfWork: NotificationEventUnitOfWork,
    event: EventEnvelope,
  ): Promise<NotificationWorkflowResult> {
    requireEventType(event, NOTIFICATION_REQUESTED_EVENT);
    const input = payloadToInput(event);

    return unitOfWork.withTransaction(async (transaction) => {
      const inserted = await transaction.markEventProcessed(event, this.now());
      if (!inserted) {
        return {
          duplicate: true,
          created: false,
          delivered: false,
          request: null,
          event: null,
        };
      }
      const request = await transaction.findByIdentity(input);
      if (!request) {
        throw new Error(
          'NotificationRequested has no local notification request',
        );
      }
      const delivered = await transaction.markDelivered(input);
      if (!delivered) {
        throw new Error(
          'notification request could not transition to delivered',
        );
      }
      return {
        duplicate: false,
        created: false,
        delivered: true,
        request: { ...request, status: 'delivered' },
        event: null,
      };
    });
  }
}

function shipmentCreatedPayload(event: EventEnvelope): {
  order_id: string;
  shipment_id: string;
  shipment_status: string;
} {
  const payload = event.payload;
  if (
    typeof payload.order_id !== 'string' ||
    typeof payload.shipment_id !== 'string' ||
    typeof payload.shipment_status !== 'string'
  ) {
    throw new Error('ShipmentCreated payload is invalid');
  }
  return {
    order_id: payload.order_id,
    shipment_id: payload.shipment_id,
    shipment_status: payload.shipment_status,
  };
}

function payloadToInput(event: EventEnvelope): CreateNotificationRequest {
  const payload = event.payload;
  if (
    typeof payload.order_id !== 'string' ||
    typeof payload.shipment_id !== 'string' ||
    payload.notification_type !== 'shipment_created'
  ) {
    throw new Error('NotificationRequested payload is invalid');
  }
  return {
    orderId: payload.order_id,
    shipmentId: payload.shipment_id,
    notificationType: payload.notification_type,
  };
}

function inputToPayload(
  input: CreateNotificationRequest,
): Record<string, unknown> {
  return {
    order_id: input.orderId,
    shipment_id: input.shipmentId,
    notification_type: input.notificationType,
  };
}
