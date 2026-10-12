import type { ConfirmChannel, ConsumeMessage } from 'amqplib';

import {
  notificationMessageProperties,
  notificationRetryHeaders,
  publishNotificationMessage,
  readNotificationRetryCount,
} from './rabbitmq-retry-publication';

describe('notifications retry publication policy', () => {
  it('reads only the service-scoped retry count', () => {
    expect(
      readNotificationRetryCount({
        'x-logistics-retry-count': 3,
        'x-commerce-retry-count': 3,
      }),
    ).toBe(0);
    expect(
      readNotificationRetryCount({ 'x-notifications-retry-count': 2 }),
    ).toBe(2);
    expect(
      readNotificationRetryCount({ 'x-notifications-retry-count': 4 }),
    ).toBeNull();
  });

  it('preserves identity properties but replaces headers with bounded metadata', () => {
    const message = testMessage();
    const options = notificationMessageProperties(
      message,
      notificationRetryHeaders(1, 'transient'),
      1000,
    );

    expect(options).toMatchObject({
      messageId: '11111111-1111-1111-1111-111111111111',
      type: 'NotificationRequested',
      correlationId: '22222222-2222-2222-2222-222222222222',
      userId: 'notifications-worker',
      appId: 'notifications',
      expiration: '1000',
      CC: 'notifications.requested',
      headers: {
        'x-notifications-retry-count': 1,
        'x-notifications-retry-error-class': 'transient',
        'x-notifications-retry-last-error': 'transient',
      },
    });
    expect((options as { clusterId?: string }).clusterId).toBe(
      'notifications-cluster',
    );
    expect(
      (options.headers as Record<string, unknown>)['authorization'],
    ).toBeUndefined();
  });

  it('resolves only after the confirm callback succeeds', async () => {
    const publish = jest.fn((_exchange, _key, _body, _options, callback) => {
      callback(null);
      return true;
    });
    const channel = { publish } as unknown as ConfirmChannel;

    await publishNotificationMessage(
      channel,
      testMessageWithoutClusterId(),
      'platform.notifications.retry',
      'notifications.retry.1',
      notificationRetryHeaders(1, 'transient'),
      1000,
    );

    expect(publish).toHaveBeenCalledTimes(1);
  });
});

function testMessage(): ConsumeMessage {
  return {
    content: Buffer.from('{"event":"notification"}'),
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
      type: 'NotificationRequested',
      correlationId: '22222222-2222-2222-2222-222222222222',
      userId: 'notifications-worker',
      appId: 'notifications',
      clusterId: 'notifications-cluster',
      headers: { authorization: 'secret' },
      contentEncoding: undefined,
      priority: undefined,
      replyTo: undefined,
      expiration: undefined,
      timestamp: undefined,
    },
  };
}

function testMessageWithoutClusterId(): ConsumeMessage {
  const message = testMessage();
  message.properties.clusterId = undefined;
  return message;
}
