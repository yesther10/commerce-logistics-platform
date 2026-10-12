import type { Channel, ConfirmChannel, ConsumeMessage } from 'amqplib';

import {
  EVENTS_DEAD_LETTER_EXCHANGE,
  parseEventEnvelope,
} from './event-envelope';
import {
  notificationRetryHeaders,
  publishNotificationMessage,
  readNotificationRetryCount,
  type NotificationFailureClass,
} from './rabbitmq-retry-publication';
import type { EventEnvelope } from './event-envelope';

export { type NotificationFailureClass } from './rabbitmq-retry-publication';

export class PermanentNotificationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PermanentNotificationError';
  }
}

export interface ParsedNotificationMessage {
  event: EventEnvelope;
  retryCount: number;
}

export function parseNotificationMessage(
  message: ConsumeMessage,
): ParsedNotificationMessage | null {
  const retryCount = readNotificationRetryCount(message.properties.headers);
  if (retryCount === null) return null;

  try {
    return {
      event: parseEventEnvelope(JSON.parse(message.content.toString('utf8'))),
      retryCount,
    };
  } catch {
    return null;
  }
}

export function classifyNotificationFailure(
  error: unknown,
): Exclude<NotificationFailureClass, 'malformed'> {
  return error instanceof PermanentNotificationError
    ? 'permanent'
    : 'transient';
}

export function rejectMalformedNotification(
  consumerChannel: Channel,
  message: ConsumeMessage,
): void {
  consumerChannel.reject(message, false);
}

export function publishNotificationDeadLetter(
  publisherChannel: ConfirmChannel,
  message: ConsumeMessage,
  retryCount: number,
  failure: Exclude<NotificationFailureClass, 'malformed'>,
): Promise<void> {
  return publishNotificationMessage(
    publisherChannel,
    message,
    EVENTS_DEAD_LETTER_EXCHANGE,
    message.fields.routingKey,
    notificationRetryHeaders(retryCount, failure),
  );
}
