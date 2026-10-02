import { Injectable } from '@nestjs/common';
import { DataSource, EntityManager, IsNull } from 'typeorm';

import type { EventEnvelope } from '../../messaging/event-envelope';
import type {
  CreateNotificationRequest,
  NotificationRequestRecord,
} from '../application/notification-request.store';
import type {
  NotificationEventTransaction,
  NotificationEventUnitOfWork,
} from '../application/notification-workflow';
import {
  NOTIFICATION_REQUEST_STATUS_DELIVERED,
  NOTIFICATION_REQUEST_STATUS_REQUESTED,
  NotificationRequestEntity,
} from './notification-request.entity';
import { OutboxEventEntity } from './outbox-event.entity';

export interface NotificationOutboxStore {
  pending(limit: number): Promise<EventEnvelope[]>;
  markPublished(eventId: string): Promise<void>;
}

@Injectable()
export class TypeOrmNotificationEventStore
  implements NotificationEventUnitOfWork, NotificationOutboxStore
{
  constructor(private readonly dataSource: DataSource) {}

  withTransaction<T>(
    work: (transaction: NotificationEventTransaction) => Promise<T>,
  ): Promise<T> {
    return this.dataSource.transaction((manager) =>
      work(new TypeOrmNotificationTransaction(manager)),
    );
  }

  async pending(limit: number): Promise<EventEnvelope[]> {
    const records = await this.dataSource
      .getRepository(OutboxEventEntity)
      .find({
        where: { publishedAt: IsNull() },
        order: { occurredAt: 'ASC', eventId: 'ASC' },
        take: limit,
      });
    return records.map((record) => ({
      event_id: record.eventId,
      event_type: record.eventType,
      event_version: record.eventVersion,
      occurred_at: record.occurredAt.toISOString().replace(/\.\d{3}Z$/, 'Z'),
      correlation_id: record.correlationId,
      causation_id: record.causationId,
      payload: record.payload,
    }));
  }

  async markPublished(eventId: string): Promise<void> {
    await this.dataSource
      .getRepository(OutboxEventEntity)
      .update({ eventId, publishedAt: IsNull() }, { publishedAt: new Date() });
  }
}

class TypeOrmNotificationTransaction implements NotificationEventTransaction {
  constructor(private readonly manager: EntityManager) {}

  async markEventProcessed(
    event: EventEnvelope,
    processedAt: Date,
  ): Promise<boolean> {
    const rows = await this.manager.query(
      `
        INSERT INTO processed_events (event_id, event_type, processed_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (event_id) DO NOTHING
        RETURNING event_id
      `,
      [event.event_id, event.event_type, processedAt],
    );
    return rows.length === 1;
  }

  async findByIdentity(
    input: CreateNotificationRequest,
  ): Promise<NotificationRequestRecord | null> {
    const entity = await this.manager
      .getRepository(NotificationRequestEntity)
      .findOne({
        where: {
          orderId: input.orderId,
          shipmentId: input.shipmentId,
          notificationType: input.notificationType,
        },
      });
    return entity ? toRecord(entity) : null;
  }

  async create(
    input: CreateNotificationRequest & { id: string },
  ): Promise<NotificationRequestRecord> {
    const entity = this.manager
      .getRepository(NotificationRequestEntity)
      .create({
        id: input.id,
        orderId: input.orderId,
        shipmentId: input.shipmentId,
        notificationType: input.notificationType,
        status: NOTIFICATION_REQUEST_STATUS_REQUESTED,
      });
    return toRecord(
      await this.manager.getRepository(NotificationRequestEntity).save(entity),
    );
  }

  async addOutboxEvent(event: EventEnvelope): Promise<void> {
    const entity = this.manager.getRepository(OutboxEventEntity).create({
      eventId: event.event_id,
      eventType: event.event_type,
      eventVersion: event.event_version,
      occurredAt: new Date(event.occurred_at),
      correlationId: event.correlation_id,
      causationId: event.causation_id,
      payload: event.payload,
      publishedAt: null,
    });
    await this.manager.getRepository(OutboxEventEntity).save(entity);
  }

  async markDelivered(input: CreateNotificationRequest): Promise<boolean> {
    const result = await this.manager
      .getRepository(NotificationRequestEntity)
      .update(
        {
          orderId: input.orderId,
          shipmentId: input.shipmentId,
          notificationType: input.notificationType,
          status: NOTIFICATION_REQUEST_STATUS_REQUESTED,
        },
        { status: NOTIFICATION_REQUEST_STATUS_DELIVERED },
      );
    return result.affected === 1;
  }
}

function toRecord(
  entity: NotificationRequestEntity,
): NotificationRequestRecord {
  return {
    id: entity.id,
    orderId: entity.orderId,
    shipmentId: entity.shipmentId,
    notificationType: entity.notificationType,
    status: entity.status,
    createdAt: entity.createdAt,
    updatedAt: entity.updatedAt,
  };
}
