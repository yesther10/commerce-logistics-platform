import type { Channel } from 'amqplib';

import {
  declareNotificationsTopology,
  notificationRetryQueuesFromConfig,
} from './rabbitmq-topology';

describe('notifications RabbitMQ topology', () => {
  it('preserves the established topology when retry configuration is omitted', async () => {
    const channel = testChannel();

    await declareNotificationsTopology(channel, baseTopology());

    expect(channel.assertExchange).toHaveBeenCalledTimes(2);
    expect(channel.assertQueue).toHaveBeenCalledTimes(4);
  });

  it('adds three durable TTL queues without changing the established queues', async () => {
    const channel = testChannel();

    await declareNotificationsTopology(channel, {
      ...baseTopology(),
      retryQueues: notificationRetryQueuesFromConfig(),
    });

    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      1,
      'notifications.shipment.created',
      expect.objectContaining({
        durable: true,
        deadLetterExchange: 'platform.events.dlx',
        deadLetterRoutingKey: 'logistics.shipment.created',
      }),
    );
    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      2,
      'notifications.shipment.created.dlq',
      { durable: true },
    );
    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      3,
      'notifications.requested',
      expect.objectContaining({
        durable: true,
        deadLetterExchange: 'platform.events.dlx',
        deadLetterRoutingKey: 'notifications.requested',
      }),
    );
    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      4,
      'notifications.requested.dlq',
      { durable: true },
    );
    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      5,
      'notifications.retry.1',
      expect.objectContaining({
        durable: true,
        arguments: {
          'x-message-ttl': 1000,
          'x-dead-letter-exchange': 'platform.events',
        },
      }),
    );
    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      6,
      'notifications.retry.2',
      expect.objectContaining({
        arguments: {
          'x-message-ttl': 5000,
          'x-dead-letter-exchange': 'platform.events',
        },
      }),
    );
    expect(channel.assertQueue).toHaveBeenNthCalledWith(
      7,
      'notifications.retry.3',
      expect.objectContaining({
        arguments: {
          'x-message-ttl': 15000,
          'x-dead-letter-exchange': 'platform.events',
        },
      }),
    );
  });

  it('rejects retry queues that do not use the fixed delays', async () => {
    const channel = testChannel();

    await expect(
      declareNotificationsTopology(channel, {
        ...baseTopology(),
        retryQueues: [
          { queue: 'retry.1', delayMs: 1000 },
          { queue: 'retry.2', delayMs: 4000 },
          { queue: 'retry.3', delayMs: 15000 },
        ],
      }),
    ).rejects.toThrow('exactly 1000, 5000, and 15000 milliseconds');
  });

  it('reads additive queue and delay overrides while retaining defaults', () => {
    expect(
      notificationRetryQueuesFromConfig((key, fallback) =>
        key.endsWith('_QUEUE_2') ? 'custom.retry.2' : fallback,
      ),
    ).toEqual([
      { queue: 'notifications.retry.1', delayMs: 1000 },
      { queue: 'custom.retry.2', delayMs: 5000 },
      { queue: 'notifications.retry.3', delayMs: 15000 },
    ]);
  });
});

function baseTopology() {
  return {
    shipmentCreatedQueue: 'notifications.shipment.created',
    shipmentCreatedDeadLetterQueue: 'notifications.shipment.created.dlq',
    notificationRequestedQueue: 'notifications.requested',
    notificationRequestedDeadLetterQueue: 'notifications.requested.dlq',
  };
}

function testChannel() {
  return {
    assertExchange: jest.fn().mockResolvedValue({}),
    assertQueue: jest.fn().mockResolvedValue({}),
    bindQueue: jest.fn().mockResolvedValue({}),
  } as unknown as jest.Mocked<Channel>;
}
