import * as amqp from 'amqplib';
import type { ConsumeMessage } from 'amqplib';

import {
  EVENTS_DEAD_LETTER_EXCHANGE,
  NOTIFICATION_REQUESTED_EVENT,
  NOTIFICATION_REQUESTED_ROUTING_KEY,
  NOTIFICATIONS_RETRY_EXCHANGE,
  notificationsRetryRoutingKey,
} from './event-envelope';
import {
  declareNotificationsTopology,
  type QueueTopology,
} from './rabbitmq-topology';
import { publishNotificationMessage } from './rabbitmq-retry-publication';

describe('notifications RabbitMQ retry topology', () => {
  it('returns a message after each TTL and preserves final-DLQ identity', async () => {
    if (process.env.RABBITMQ_INTEGRATION !== '1') {
      return;
    }
    const connection = await amqp.connect(
      process.env.RABBITMQ_URL ??
        'amqp://dev_rabbitmq:dev_rabbitmq_password@127.0.0.1:5673/',
    );
    const channel = await connection.createConfirmChannel();
    const suffix = `${Date.now()}`;
    const retryExchange = `${NOTIFICATIONS_RETRY_EXCHANGE}.verify.${suffix}`;
    const topology: QueueTopology = {
      shipmentCreatedQueue: `notifications.verify.shipment.${suffix}`,
      shipmentCreatedDeadLetterQueue: `notifications.verify.shipment.dlq.${suffix}`,
      notificationRequestedQueue: `notifications.verify.requested.${suffix}`,
      notificationRequestedDeadLetterQueue: `notifications.verify.requested.dlq.${suffix}`,
      retryQueues: [
        { queue: `notifications.verify.retry.1.${suffix}`, delayMs: 1000 },
        { queue: `notifications.verify.retry.2.${suffix}`, delayMs: 5000 },
        { queue: `notifications.verify.retry.3.${suffix}`, delayMs: 15000 },
      ],
    };
    const retryQueues = topology.retryQueues ?? [];
    const body = Buffer.from(
      JSON.stringify({
        event_id: '11111111-1111-1111-1111-111111111111',
        event_type: NOTIFICATION_REQUESTED_EVENT,
        event_version: 1,
        occurred_at: '2026-01-01T12:00:00Z',
        correlation_id: '22222222-2222-2222-2222-222222222222',
        causation_id: null,
        payload: { order_id: 'verification-order' },
      }),
    );

    try {
      await declareNotificationsTopology(channel, topology);
      await channel.assertExchange(retryExchange, 'topic', {
        durable: false,
        autoDelete: true,
      });
      for (const [index, { queue }] of retryQueues.entries()) {
        await channel.bindQueue(
          queue,
          retryExchange,
          notificationsRetryRoutingKey(index + 1),
        );
      }
      for (const [index, delayMs] of [1000, 5000, 15000].entries()) {
        const startedAt = Date.now();
        await publishNotificationMessage(
          channel,
          testMessage(body, NOTIFICATION_REQUESTED_ROUTING_KEY, retryExchange),
          retryExchange,
          notificationsRetryRoutingKey(index + 1),
          { 'x-notifications-retry-count': index + 1 },
          delayMs,
        );
        const delivery = await getEventually(
          channel,
          topology.notificationRequestedQueue,
        );
        expect(Date.now() - startedAt).toBeGreaterThanOrEqual(delayMs - 10);
        expect(delivery.content).toEqual(body);
        expect(delivery.properties.messageId).toBe(
          '11111111-1111-1111-1111-111111111111',
        );
        expect(delivery.properties.type).toBe(NOTIFICATION_REQUESTED_EVENT);
        expect(delivery.properties.correlationId).toBe(
          '22222222-2222-2222-2222-222222222222',
        );
        expect(delivery.properties.userId).toBe('dev_rabbitmq');
        expect(delivery.properties.appId).toBe('notifications');
        expect(delivery.properties.clusterId).toBe('notifications-cluster');
      }
      await channel.purgeQueue(topology.notificationRequestedDeadLetterQueue);
      await publishNotificationMessage(
        channel,
        testMessage(body, NOTIFICATION_REQUESTED_ROUTING_KEY, retryExchange),
        EVENTS_DEAD_LETTER_EXCHANGE,
        NOTIFICATION_REQUESTED_ROUTING_KEY,
        { 'x-notifications-retry-count': 3 },
      );
      const deadLetter = await getEventually(
        channel,
        topology.notificationRequestedDeadLetterQueue,
      );
      expect(deadLetter.properties.messageId).toBe(
        '11111111-1111-1111-1111-111111111111',
      );
      expect(deadLetter.properties.type).toBe(NOTIFICATION_REQUESTED_EVENT);
      expect(deadLetter.properties.correlationId).toBe(
        '22222222-2222-2222-2222-222222222222',
      );
      expect(deadLetter.properties.userId).toBe('dev_rabbitmq');
      expect(deadLetter.properties.appId).toBe('notifications');
      expect(deadLetter.properties.clusterId).toBe('notifications-cluster');
      expect(deadLetter.content).toEqual(body);
    } finally {
      for (const queue of [
        topology.shipmentCreatedQueue,
        topology.shipmentCreatedDeadLetterQueue,
        topology.notificationRequestedQueue,
        topology.notificationRequestedDeadLetterQueue,
        ...retryQueues.map(({ queue }) => queue),
      ]) {
        await channel.deleteQueue(queue);
      }
      await channel.deleteExchange(retryExchange);
      await channel.close();
      await connection.close();
    }
  }, 30000);
});

function testMessage(
  body: Buffer,
  routingKey: string,
  exchange: string,
): ConsumeMessage {
  return {
    content: body,
    fields: {
      consumerTag: 'verification-consumer',
      deliveryTag: 1,
      redelivered: false,
      exchange,
      routingKey,
    },
    properties: {
      contentType: 'application/json',
      deliveryMode: 2,
      messageId: '11111111-1111-1111-1111-111111111111',
      type: NOTIFICATION_REQUESTED_EVENT,
      correlationId: '22222222-2222-2222-2222-222222222222',
      userId: 'dev_rabbitmq',
      appId: 'notifications',
      clusterId: 'notifications-cluster',
      headers: {},
      contentEncoding: undefined,
      priority: undefined,
      replyTo: undefined,
      expiration: undefined,
      timestamp: undefined,
    },
  };
}

async function getEventually(
  channel: amqp.Channel,
  queue: string,
): Promise<amqp.GetMessage> {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const delivery = await channel.get(queue, { noAck: true });
    if (delivery) return delivery;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  throw new Error(`timed out waiting for ${queue}`);
}
