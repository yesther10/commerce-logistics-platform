import type { Channel } from 'amqplib';

import {
  EVENTS_DEAD_LETTER_EXCHANGE,
  EVENTS_EXCHANGE,
  NOTIFICATIONS_RETRY_EXCHANGE,
  notificationsRetryRoutingKey,
  NOTIFICATION_REQUESTED_ROUTING_KEY,
  SHIPMENT_CREATED_ROUTING_KEY,
} from './event-envelope';

export interface QueueTopology {
  shipmentCreatedQueue: string;
  shipmentCreatedDeadLetterQueue: string;
  notificationRequestedQueue: string;
  notificationRequestedDeadLetterQueue: string;
  retryQueues?: Array<{ queue: string; delayMs: number }>;
}

export const NOTIFICATIONS_RETRY_DELAYS_MS = [1000, 5000, 15000] as const;

export function notificationRetryQueuesFromConfig(
  get: (key: string, fallback: string | number) => unknown = (_key, fallback) =>
    fallback,
): Array<{ queue: string; delayMs: number }> {
  return NOTIFICATIONS_RETRY_DELAYS_MS.map((delayMs, index) => ({
    queue: String(
      get(
        `RABBITMQ_NOTIFICATIONS_RETRY_QUEUE_${index + 1}`,
        `notifications.retry.${index + 1}`,
      ),
    ),
    delayMs: Number(
      get(`RABBITMQ_NOTIFICATIONS_RETRY_DELAY_${index + 1}_MS`, delayMs),
    ),
  }));
}

export async function declareNotificationsTopology(
  channel: Channel,
  topology: QueueTopology,
): Promise<void> {
  await channel.assertExchange(EVENTS_EXCHANGE, 'topic', { durable: true });
  await channel.assertExchange(EVENTS_DEAD_LETTER_EXCHANGE, 'topic', {
    durable: true,
  });
  await declareQueue(
    channel,
    topology.shipmentCreatedQueue,
    topology.shipmentCreatedDeadLetterQueue,
    SHIPMENT_CREATED_ROUTING_KEY,
  );
  await declareQueue(
    channel,
    topology.notificationRequestedQueue,
    topology.notificationRequestedDeadLetterQueue,
    NOTIFICATION_REQUESTED_ROUTING_KEY,
  );
  if (topology.retryQueues) {
    validateRetryQueues(topology.retryQueues);
    await channel.assertExchange(NOTIFICATIONS_RETRY_EXCHANGE, 'topic', {
      durable: true,
    });
    for (const [index, retryQueue] of topology.retryQueues.entries()) {
      await channel.assertQueue(retryQueue.queue, {
        durable: true,
        arguments: {
          'x-message-ttl': retryQueue.delayMs,
          'x-dead-letter-exchange': EVENTS_EXCHANGE,
        },
      });
      await channel.bindQueue(
        retryQueue.queue,
        NOTIFICATIONS_RETRY_EXCHANGE,
        notificationsRetryRoutingKey(index + 1),
      );
    }
  }
}

function validateRetryQueues(
  retryQueues: Array<{ queue: string; delayMs: number }>,
): void {
  if (retryQueues.length !== 3) {
    throw new Error('notifications require exactly three retry queues');
  }
  for (const [index, retryQueue] of retryQueues.entries()) {
    if (
      !retryQueue.queue ||
      !Number.isSafeInteger(retryQueue.delayMs) ||
      retryQueue.delayMs !== NOTIFICATIONS_RETRY_DELAYS_MS[index]
    ) {
      throw new Error(
        'notifications retry delays must be exactly 1000, 5000, and 15000 milliseconds',
      );
    }
  }
}

async function declareQueue(
  channel: Channel,
  queue: string,
  deadLetterQueue: string,
  routingKey: string,
): Promise<void> {
  await channel.assertQueue(queue, {
    durable: true,
    deadLetterExchange: EVENTS_DEAD_LETTER_EXCHANGE,
    deadLetterRoutingKey: routingKey,
  });
  await channel.bindQueue(queue, EVENTS_EXCHANGE, routingKey);
  await channel.assertQueue(deadLetterQueue, { durable: true });
  await channel.bindQueue(
    deadLetterQueue,
    EVENTS_DEAD_LETTER_EXCHANGE,
    routingKey,
  );
}
