import type { Channel, ConfirmChannel, ConsumeMessage } from 'amqplib';

import { NOTIFICATION_REQUESTED_EVENT } from './event-envelope';
import {
  classifyNotificationFailure,
  parseNotificationMessage,
  PermanentNotificationError,
  publishNotificationDeadLetter,
  rejectMalformedNotification,
} from './rabbitmq-worker-classification';

describe('RabbitMQ worker classification', () => {
  it('returns no event for malformed bodies without reconstructing them', () => {
    expect(
      parseNotificationMessage(testMessage({ content: Buffer.from('{') })),
    ).toBeNull();
  });

  it('reads only valid notifications retry metadata', () => {
    expect(
      parseNotificationMessage(
        testMessage({ headers: { 'x-logistics-retry-count': 2 } }),
      )?.retryCount,
    ).toBe(0);
    expect(
      parseNotificationMessage(
        testMessage({ headers: { 'x-notifications-retry-count': 2 } }),
      )?.retryCount,
    ).toBe(2);
    expect(
      parseNotificationMessage(
        testMessage({ headers: { 'x-notifications-retry-count': 'invalid' } }),
      ),
    ).toBeNull();
  });

  it('classifies permanent and transient processing failures', () => {
    expect(
      classifyNotificationFailure(new PermanentNotificationError('invalid')),
    ).toBe('permanent');
    expect(classifyNotificationFailure(new Error('database unavailable'))).toBe(
      'transient',
    );
  });

  it('rejects malformed deliveries through the broker without publishing', () => {
    const consumer = { reject: jest.fn() } as unknown as Channel;
    rejectMalformedNotification(consumer, testMessage());
    expect(consumer.reject).toHaveBeenCalledWith(expect.anything(), false);
  });

  it('publishes permanent failures directly to the final DLQ with bounded metadata', async () => {
    const publish = jest.fn((_exchange, _key, _body, _options, callback) => {
      callback(null);
      return true;
    });
    const publisher = { publish } as unknown as ConfirmChannel;

    await publishNotificationDeadLetter(
      publisher,
      testMessage({ headers: { authorization: 'secret' } }),
      0,
      'permanent',
    );

    expect(publish).toHaveBeenCalledWith(
      'platform.events.dlx',
      'notifications.requested',
      expect.any(Buffer),
      expect.objectContaining({
        headers: {
          'x-notifications-retry-count': 0,
          'x-notifications-retry-error-class': 'permanent',
          'x-notifications-retry-last-error': 'permanent',
        },
      }),
      expect.any(Function),
    );
  });
});

function testMessage(
  input: {
    content?: Buffer;
    headers?: Record<string, unknown>;
  } = {},
): ConsumeMessage {
  return {
    content:
      input.content ??
      Buffer.from(
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
