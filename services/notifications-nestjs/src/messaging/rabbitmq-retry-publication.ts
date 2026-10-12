import type * as amqp from 'amqplib';
import type { ConfirmChannel, ConsumeMessage } from 'amqplib';

export type NotificationFailureClass = 'malformed' | 'permanent' | 'transient';

export function readNotificationRetryCount(headers: unknown): number | null {
  if (!headers || typeof headers !== 'object') return 0;
  const value = (headers as Record<string, unknown>)[
    'x-notifications-retry-count'
  ];
  if (value === undefined) return 0;
  const count =
    typeof value === 'number'
      ? value
      : typeof value === 'string' && /^\d+$/.test(value)
        ? Number(value)
        : Number.NaN;
  return Number.isInteger(count) && count >= 0 && count <= 3 ? count : null;
}

export function notificationRetryHeaders(
  retryCount: number,
  failure: NotificationFailureClass,
): Record<string, number | string> {
  return {
    'x-notifications-retry-count': retryCount,
    'x-notifications-retry-error-class': failure,
    'x-notifications-retry-last-error': failure,
  };
}

export function notificationMessageProperties(
  message: ConsumeMessage,
  headers: Record<string, number | string>,
  retryDelayMs?: number,
): amqp.Options.Publish {
  const properties = message.properties;
  const options: amqp.Options.Publish = {
    contentType: properties.contentType,
    contentEncoding: properties.contentEncoding,
    deliveryMode: properties.deliveryMode,
    priority: properties.priority,
    correlationId: properties.correlationId,
    replyTo: properties.replyTo,
    messageId: properties.messageId,
    timestamp: properties.timestamp,
    type: properties.type,
    userId: properties.userId,
    appId: properties.appId,
    headers,
  };
  if (properties.clusterId !== undefined) {
    (options as amqp.Options.Publish & { clusterId?: string }).clusterId =
      properties.clusterId;
  }
  if (retryDelayMs !== undefined) {
    options.expiration = String(retryDelayMs);
    options.CC = message.fields.routingKey;
  } else {
    options.expiration = properties.expiration;
  }
  return options;
}

export function publishNotificationMessage(
  channel: ConfirmChannel,
  message: ConsumeMessage,
  exchange: string,
  routingKey: string,
  headers: Record<string, number | string>,
  retryDelayMs?: number,
): Promise<void> {
  const options = notificationMessageProperties(message, headers, retryDelayMs);
  return publishConfirmed(
    channel,
    exchange,
    routingKey,
    message.content,
    options,
  );
}

function publishConfirmed(
  channel: ConfirmChannel,
  exchange: string,
  routingKey: string,
  body: Buffer,
  options: amqp.Options.Publish,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const clusterId = (options as amqp.Options.Publish & { clusterId?: string })
      .clusterId;
    const callback = (error?: Error | null): void =>
      error ? reject(error) : resolve();

    if (clusterId === undefined) {
      channel.publish(exchange, routingKey, body, options, callback);
      return;
    }

    // amqplib 0.10.9 receives clusterId but drops it in its public publish()
    // helper. Restore the property in the same encoded fields before sending.
    const internalChannel = channel as unknown as {
      pushConfirmCallback(callback: (error?: Error | null) => void): void;
      sendMessage(
        fields: Record<string, unknown>,
        properties: Record<string, unknown>,
        body: Buffer,
      ): boolean;
    };
    if (
      typeof internalChannel.pushConfirmCallback !== 'function' ||
      typeof internalChannel.sendMessage !== 'function'
    ) {
      channel.publish(exchange, routingKey, body, options, callback);
      return;
    }
    internalChannel.pushConfirmCallback(callback);
    const fieldsAndProperties = publishFields(exchange, routingKey, options);
    fieldsAndProperties.clusterId = clusterId;
    internalChannel.sendMessage(fieldsAndProperties, fieldsAndProperties, body);
  });
}

function publishFields(
  exchange: string,
  routingKey: string,
  options: amqp.Options.Publish,
): Record<string, unknown> {
  const headers = { ...(options.headers ?? {}) } as Record<string, unknown>;
  if (options.CC !== undefined) {
    headers.CC = Array.isArray(options.CC)
      ? options.CC.map(String)
      : [String(options.CC)];
  }
  if (options.BCC !== undefined) {
    headers.BCC = Array.isArray(options.BCC)
      ? options.BCC.map(String)
      : [String(options.BCC)];
  }
  const deliveryMode = options.persistent
    ? 2
    : typeof options.deliveryMode === 'number'
      ? options.deliveryMode
      : options.deliveryMode
        ? 2
        : undefined;
  return {
    exchange,
    routingKey,
    mandatory: !!options.mandatory,
    immediate: false,
    ticket: undefined,
    contentType: options.contentType,
    contentEncoding: options.contentEncoding,
    headers,
    deliveryMode,
    priority: options.priority,
    correlationId: options.correlationId,
    replyTo: options.replyTo,
    expiration:
      options.expiration === undefined ? undefined : String(options.expiration),
    messageId: options.messageId,
    timestamp: options.timestamp,
    type: options.type,
    userId: options.userId,
    appId: options.appId,
    clusterId: undefined,
  };
}
