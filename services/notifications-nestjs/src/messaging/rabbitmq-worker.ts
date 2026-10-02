import {
  Inject,
  Injectable,
  Logger,
  OnApplicationShutdown,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as amqp from 'amqplib';
import type {
  Channel,
  ChannelModel,
  ConfirmChannel,
  ConsumeMessage,
} from 'amqplib';

import {
  parseEventEnvelope,
  routingKeyFor,
  serializeEventEnvelope,
  SHIPMENT_CREATED_EVENT,
  NOTIFICATION_REQUESTED_EVENT,
} from './event-envelope';
import {
  declareNotificationsTopology,
  type QueueTopology,
} from './rabbitmq-topology';
import { NotificationEventProcessor } from '../notifications/application/notification-event-processor';
import {
  NOTIFICATION_EVENT_UNIT_OF_WORK,
  type NotificationEventUnitOfWork,
} from '../notifications/application/notification-workflow';
import type { NotificationOutboxStore } from '../notifications/persistence/typeorm-notification-event.store';

@Injectable()
export class RabbitMqWorker implements OnApplicationShutdown {
  private readonly logger = new Logger(RabbitMqWorker.name);
  private connection: ChannelModel | null = null;
  private consumerChannel: Channel | null = null;
  private publisherChannel: ConfirmChannel | null = null;
  private outboxTimer: NodeJS.Timeout | null = null;

  constructor(
    private readonly config: ConfigService,
    private readonly processor: NotificationEventProcessor,
    @Inject(NOTIFICATION_EVENT_UNIT_OF_WORK)
    private readonly outboxStore: NotificationOutboxStore &
      NotificationEventUnitOfWork,
  ) {}

  async start(): Promise<void> {
    if (!this.isEnabled() || this.connection) {
      return;
    }
    const connection = await amqp.connect(
      this.config.getOrThrow<string>('RABBITMQ_URL'),
    );
    this.connection = connection;
    connection.on('error', (error) =>
      this.logger.error(`RabbitMQ connection error: ${error.message}`),
    );
    const consumerChannel = await connection.createChannel();
    const publisherChannel = await connection.createConfirmChannel();
    this.consumerChannel = consumerChannel;
    this.publisherChannel = publisherChannel;
    const topology = this.topology();
    await declareNotificationsTopology(consumerChannel, topology);
    await consumerChannel.prefetch(1);
    await consumerChannel.consume(
      topology.shipmentCreatedQueue,
      (message) => {
        if (message) void this.handleShipmentCreated(message);
      },
      { noAck: false },
    );
    await consumerChannel.consume(
      topology.notificationRequestedQueue,
      (message) => {
        if (message) void this.handleNotificationRequested(message);
      },
      { noAck: false },
    );
    await this.publishPendingOutbox();
    const interval = this.config.get<number>('OUTBOX_POLL_INTERVAL', 1000);
    this.outboxTimer = setInterval(
      () => void this.publishPendingOutbox(),
      interval,
    );
    this.logger.log('RabbitMQ consumers and outbox publisher started');
  }

  async onApplicationShutdown(): Promise<void> {
    if (this.outboxTimer) clearInterval(this.outboxTimer);
    await this.consumerChannel?.close();
    await this.publisherChannel?.close();
    await this.connection?.close();
    this.outboxTimer = null;
    this.consumerChannel = null;
    this.publisherChannel = null;
    this.connection = null;
  }

  private async handleShipmentCreated(message: ConsumeMessage): Promise<void> {
    await this.handleMessage(message, (event) => {
      if (event.event_type !== SHIPMENT_CREATED_EVENT) {
        throw new Error(`unexpected event type ${event.event_type}`);
      }
      return this.processor.processShipmentCreated(event);
    });
  }

  private async handleNotificationRequested(
    message: ConsumeMessage,
  ): Promise<void> {
    await this.handleMessage(message, (event) => {
      if (event.event_type !== NOTIFICATION_REQUESTED_EVENT) {
        throw new Error(`unexpected event type ${event.event_type}`);
      }
      return this.processor.processNotificationRequested(event);
    });
  }

  private async handleMessage(
    message: ConsumeMessage,
    handler: (event: ReturnType<typeof parseEventEnvelope>) => Promise<unknown>,
  ): Promise<void> {
    try {
      const event = parseEventEnvelope(
        JSON.parse(message.content.toString('utf8')),
      );
      await handler(event);
      this.consumerChannel?.ack(message);
    } catch (error) {
      this.logger.error(error instanceof Error ? error.message : String(error));
      this.consumerChannel?.reject(message, false);
    }
  }

  private async publishPendingOutbox(): Promise<void> {
    if (!this.publisherChannel) return;
    try {
      const events = await this.outboxStore.pending(
        this.config.get<number>('OUTBOX_BATCH_SIZE', 100),
      );
      for (const event of events) {
        await this.publish(event);
        await this.outboxStore.markPublished(event.event_id);
      }
    } catch (error) {
      if (this.connection) {
        this.logger.error(
          `publish notifications outbox: ${error instanceof Error ? error.message : String(error)}`,
        );
      }
    }
  }

  private publish(event: ReturnType<typeof parseEventEnvelope>): Promise<void> {
    const channel = this.publisherChannel;
    if (!channel)
      return Promise.reject(new Error('RabbitMQ publisher is not started'));
    return new Promise((resolve, reject) => {
      channel.publish(
        'platform.events',
        routingKeyFor(event.event_type),
        serializeEventEnvelope(event),
        {
          contentType: 'application/json',
          persistent: true,
          messageId: event.event_id,
          type: event.event_type,
          correlationId: event.correlation_id,
        },
        (error) => (error ? reject(error) : resolve()),
      );
    });
  }

  private isEnabled(): boolean {
    return (
      this.config.get<string>('RABBITMQ_ENABLED', 'false').toLowerCase() ===
      'true'
    );
  }

  private topology(): QueueTopology {
    return {
      shipmentCreatedQueue: this.config.get(
        'RABBITMQ_SHIPMENT_CREATED_QUEUE',
        'notifications.logistics.shipment.created',
      ),
      shipmentCreatedDeadLetterQueue: this.config.get(
        'RABBITMQ_SHIPMENT_CREATED_DLQ',
        'notifications.logistics.shipment.created.dlq',
      ),
      notificationRequestedQueue: this.config.get(
        'RABBITMQ_NOTIFICATION_REQUESTED_QUEUE',
        'notifications.requested',
      ),
      notificationRequestedDeadLetterQueue: this.config.get(
        'RABBITMQ_NOTIFICATION_REQUESTED_DLQ',
        'notifications.requested.dlq',
      ),
    };
  }
}
