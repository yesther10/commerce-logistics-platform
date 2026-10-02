import { randomUUID } from 'node:crypto';

export const EVENTS_EXCHANGE = 'platform.events';
export const EVENTS_DEAD_LETTER_EXCHANGE = 'platform.events.dlx';
export const SHIPMENT_CREATED_EVENT = 'ShipmentCreated';
export const NOTIFICATION_REQUESTED_EVENT = 'NotificationRequested';
export const SHIPMENT_CREATED_ROUTING_KEY = 'logistics.shipment.created';
export const NOTIFICATION_REQUESTED_ROUTING_KEY = 'notifications.requested';

export interface EventEnvelope<
  TPayload extends Record<string, unknown> = Record<string, unknown>,
> {
  event_id: string;
  event_type: string;
  event_version: number;
  occurred_at: string;
  correlation_id: string;
  causation_id: string | null;
  payload: TPayload;
}

export function parseEventEnvelope(value: unknown): EventEnvelope {
  if (!isRecord(value)) {
    throw new Error('event envelope must be an object');
  }
  const envelope = value as Partial<EventEnvelope>;
  if (
    typeof envelope.event_id !== 'string' ||
    typeof envelope.event_type !== 'string' ||
    typeof envelope.event_version !== 'number' ||
    typeof envelope.occurred_at !== 'string' ||
    typeof envelope.correlation_id !== 'string' ||
    (envelope.causation_id !== null &&
      typeof envelope.causation_id !== 'string') ||
    !isRecord(envelope.payload)
  ) {
    throw new Error('event envelope has an invalid shape');
  }
  if (!isUuid(envelope.event_id) || !isUuid(envelope.correlation_id)) {
    throw new Error('event_id and correlation_id must be UUIDs');
  }
  if (envelope.causation_id !== null && !isUuid(envelope.causation_id)) {
    throw new Error('causation_id must be a UUID or null');
  }
  if (Number.isNaN(Date.parse(envelope.occurred_at))) {
    throw new Error('occurred_at must be an ISO 8601 timestamp');
  }
  return envelope as EventEnvelope;
}

export function requireEventType(
  envelope: EventEnvelope,
  eventType: string,
): void {
  parseEventEnvelope(envelope);
  if (envelope.event_type !== eventType) {
    throw new Error(
      `expected event type ${eventType}, got ${envelope.event_type}`,
    );
  }
  if (envelope.event_version !== 1) {
    throw new Error(`unsupported event version ${envelope.event_version}`);
  }
}

export function serializeEventEnvelope(envelope: EventEnvelope): Buffer {
  return Buffer.from(JSON.stringify(envelope));
}

export function newEventEnvelope(
  eventType: string,
  correlationId: string,
  causationId: string,
  payload: Record<string, unknown>,
  now: Date = new Date(),
  eventId: string = randomUUID(),
): EventEnvelope {
  return {
    event_id: eventId,
    event_type: eventType,
    event_version: 1,
    occurred_at: now.toISOString().replace(/\.\d{3}Z$/, 'Z'),
    correlation_id: correlationId,
    causation_id: causationId,
    payload,
  };
}

export function routingKeyFor(eventType: string): string {
  switch (eventType) {
    case SHIPMENT_CREATED_EVENT:
      return SHIPMENT_CREATED_ROUTING_KEY;
    case NOTIFICATION_REQUESTED_EVENT:
      return NOTIFICATION_REQUESTED_ROUTING_KEY;
    default:
      throw new Error(`unsupported event type ${eventType}`);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isUuid(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
    value,
  );
}
