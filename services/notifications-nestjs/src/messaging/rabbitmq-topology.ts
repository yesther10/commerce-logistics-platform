import type { Channel } from 'amqplib';

import {
  EVENTS_DEAD_LETTER_EXCHANGE,
  EVENTS_EXCHANGE,
  NOTIFICATION_REQUESTED_ROUTING_KEY,
  SHIPMENT_CREATED_ROUTING_KEY,
} from './event-envelope';

export interface QueueTopology {
  shipmentCreatedQueue: string;
  shipmentCreatedDeadLetterQueue: string;
  notificationRequestedQueue: string;
  notificationRequestedDeadLetterQueue: string;
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
