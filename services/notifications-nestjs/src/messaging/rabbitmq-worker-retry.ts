import type { Channel, ConfirmChannel, ConsumeMessage } from 'amqplib';

import {
  NOTIFICATIONS_RETRY_EXCHANGE,
  notificationsRetryRoutingKey,
} from './event-envelope';
import type { EventEnvelope } from './event-envelope';
import {
  classifyNotificationFailure,
  parseNotificationMessage,
  publishNotificationDeadLetter,
  rejectMalformedNotification,
} from './rabbitmq-worker-classification';
import {
  notificationRetryHeaders,
  publishNotificationMessage,
} from './rabbitmq-retry-publication';

export interface NotificationRetryQueue {
  queue: string;
  delayMs: number;
}

export interface NotificationRetryDependencies {
  consumerChannel: Channel | null;
  publisherChannel: ConfirmChannel | null;
  retryQueues: readonly NotificationRetryQueue[];
}

export type NotificationEventHandler = (
  event: EventEnvelope,
) => Promise<unknown>;

export async function handleNotificationMessage(
  message: ConsumeMessage,
  handler: NotificationEventHandler,
  dependencies: NotificationRetryDependencies,
): Promise<void> {
  const parsed = parseNotificationMessage(message);
  if (!parsed) {
    if (!dependencies.consumerChannel) {
      throw new Error('RabbitMQ consumer is not started');
    }
    rejectMalformedNotification(dependencies.consumerChannel, message);
    return;
  }

  let failure: 'permanent' | 'transient' | null = null;
  try {
    await handler(parsed.event);
  } catch (error) {
    failure = classifyNotificationFailure(error);
  }

  if (!failure) {
    try {
      acknowledgeMessage(dependencies.consumerChannel, message);
    } catch (error) {
      requeueMessage(dependencies.consumerChannel, message);
      throw error;
    }
    return;
  }

  try {
    if (failure === 'transient' && parsed.retryCount < 3) {
      await publishNotificationRetry(
        dependencies,
        message,
        parsed.retryCount + 1,
      );
    } else {
      await publishNotificationFinalOutcome(
        dependencies,
        message,
        parsed.retryCount,
        failure,
      );
    }
    acknowledgeMessage(dependencies.consumerChannel, message);
  } catch (error) {
    requeueMessage(dependencies.consumerChannel, message);
    throw error;
  }
}

async function publishNotificationRetry(
  dependencies: NotificationRetryDependencies,
  message: ConsumeMessage,
  retryCount: number,
): Promise<void> {
  const retryQueue = dependencies.retryQueues[retryCount - 1];
  if (!retryQueue) {
    throw new Error(`unsupported notifications retry count ${retryCount}`);
  }
  if (!dependencies.publisherChannel) {
    throw new Error('RabbitMQ publisher is not started');
  }
  await publishNotificationMessage(
    dependencies.publisherChannel,
    message,
    NOTIFICATIONS_RETRY_EXCHANGE,
    notificationsRetryRoutingKey(retryCount),
    notificationRetryHeaders(retryCount, 'transient'),
    retryQueue.delayMs,
  );
}

function publishNotificationFinalOutcome(
  dependencies: NotificationRetryDependencies,
  message: ConsumeMessage,
  retryCount: number,
  failure: 'permanent' | 'transient',
): Promise<void> {
  if (!dependencies.publisherChannel) {
    return Promise.reject(new Error('RabbitMQ publisher is not started'));
  }
  return publishNotificationDeadLetter(
    dependencies.publisherChannel,
    message,
    retryCount,
    failure,
  );
}

function acknowledgeMessage(
  consumerChannel: Channel | null,
  message: ConsumeMessage,
): void {
  if (!consumerChannel) {
    throw new Error('RabbitMQ consumer is not started');
  }
  consumerChannel.ack(message);
}

function requeueMessage(
  consumerChannel: Channel | null,
  message: ConsumeMessage,
): void {
  try {
    consumerChannel?.nack(message, false, true);
  } catch {
    // A closed channel leaves the delivery unacknowledged for broker redelivery.
  }
}
