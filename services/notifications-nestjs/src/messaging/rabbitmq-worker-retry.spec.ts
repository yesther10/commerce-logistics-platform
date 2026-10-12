import type { Channel, ConfirmChannel, ConsumeMessage } from 'amqplib';

import { NOTIFICATION_REQUESTED_EVENT } from './event-envelope';
import {
  handleNotificationMessage,
  type NotificationRetryDependencies,
} from './rabbitmq-worker-retry';

describe('RabbitMQ worker bounded retry state machine', () => {
  it.each([
    [0, 'notifications.retry.1', '1000'],
    [1, 'notifications.retry.2', '5000'],
    [2, 'notifications.retry.3', '15000'],
  ])(
    'publishes transient retry count %i to the next TTL queue',
    async (retryCount, routingKey, expiration) => {
      const { dependencies, publisher, consumer } = testDependencies();

      await handleNotificationMessage(
        testMessage({ headers: { 'x-notifications-retry-count': retryCount } }),
        async () => {
          throw new Error('database unavailable');
        },
        dependencies,
      );

      const publication = publisher.publish.mock.calls[0];
      expect(publication[0]).toBe('platform.notifications.retry');
      expect(publication[1]).toBe(routingKey);
      expect(publication[3]).toMatchObject({
        expiration,
        CC: 'notifications.requested',
        headers: {
          'x-notifications-retry-count': retryCount + 1,
          'x-notifications-retry-error-class': 'transient',
          'x-notifications-retry-last-error': 'transient',
        },
      });
      expect(consumer.ack).toHaveBeenCalledTimes(1);
    },
  );

  it('publishes an exhausted transient failure to the final DLQ', async () => {
    const { dependencies, publisher, consumer } = testDependencies();

    await handleNotificationMessage(
      testMessage({ headers: { 'x-notifications-retry-count': 3 } }),
      async () => {
        throw new Error('database unavailable');
      },
      dependencies,
    );

    expect(publisher.publish.mock.calls[0][0]).toBe('platform.events.dlx');
    expect(publisher.publish.mock.calls[0][1]).toBe('notifications.requested');
    expect(publisher.publish.mock.calls[0][3]?.headers).toEqual({
      'x-notifications-retry-count': 3,
      'x-notifications-retry-error-class': 'transient',
      'x-notifications-retry-last-error': 'transient',
    });
    expect(consumer.ack).toHaveBeenCalledTimes(1);
  });

  it('publishes before ACK and keeps the original delivery visible on failure', async () => {
    const { dependencies, publisher, consumer } = testDependencies();
    publisher.publish.mockImplementationOnce(
      (_exchange, _key, _body, _options, callback) => {
        callback?.(new Error('publisher confirmation failed'), {});
        return true;
      },
    );

    await expect(
      handleNotificationMessage(
        testMessage(),
        async () => {
          throw new Error('temporary failure');
        },
        dependencies,
      ),
    ).rejects.toThrow('publisher confirmation failed');
    expect(consumer.ack).not.toHaveBeenCalled();
    expect(consumer.nack).toHaveBeenCalledWith(expect.anything(), false, true);
  });

  it('does not ACK twice when the first ACK fails', async () => {
    const { dependencies, consumer } = testDependencies();
    consumer.ack.mockImplementationOnce(() => {
      throw new Error('consumer channel closed');
    });

    await expect(
      handleNotificationMessage(
        testMessage(),
        async () => ({ duplicate: true }),
        dependencies,
      ),
    ).rejects.toThrow('consumer channel closed');
    expect(consumer.ack).toHaveBeenCalledTimes(1);
    expect(consumer.nack).toHaveBeenCalledWith(expect.anything(), false, true);
  });
});

function testDependencies(): {
  dependencies: NotificationRetryDependencies;
  publisher: jest.Mocked<ConfirmChannel>;
  consumer: jest.Mocked<Channel>;
} {
  const consumer = {
    ack: jest.fn(),
    nack: jest.fn(),
    reject: jest.fn(),
  } as unknown as jest.Mocked<Channel>;
  const publisher = {
    publish: jest.fn((_exchange, _key, _body, _options, callback) => {
      callback(null);
      return true;
    }),
  } as unknown as jest.Mocked<ConfirmChannel>;
  return {
    dependencies: {
      consumerChannel: consumer,
      publisherChannel: publisher,
      retryQueues: [
        { queue: 'notifications.retry.1', delayMs: 1000 },
        { queue: 'notifications.retry.2', delayMs: 5000 },
        { queue: 'notifications.retry.3', delayMs: 15000 },
      ],
    },
    publisher,
    consumer,
  };
}

function testMessage(
  input: { headers?: Record<string, unknown> } = {},
): ConsumeMessage {
  return {
    content: Buffer.from(
      JSON.stringify({
        event_id: '11111111-1111-1111-1111-111111111111',
        event_type: NOTIFICATION_REQUESTED_EVENT,
        event_version: 1,
        occurred_at: '2026-01-01T12:00:00Z',
        correlation_id: '22222222-2222-2222-2222-222222222222',
        causation_id: null,
        payload: { order_id: 'order-1' },
      }),
    ),
    fields: {
      consumerTag: 'test-consumer',
      deliveryTag: 1,
      redelivered: false,
      exchange: 'platform.events',
      routingKey: 'notifications.requested',
    },
    properties: {
      contentType: 'application/json',
      deliveryMode: 2,
      messageId: '11111111-1111-1111-1111-111111111111',
      type: NOTIFICATION_REQUESTED_EVENT,
      correlationId: '22222222-2222-2222-2222-222222222222',
      headers: input.headers ?? {},
      contentEncoding: undefined,
      priority: undefined,
      replyTo: undefined,
      expiration: undefined,
      timestamp: undefined,
      userId: undefined,
      appId: undefined,
      clusterId: undefined,
    },
  };
}
