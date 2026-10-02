import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { randomUUID } from 'node:crypto';
import { Repository } from 'typeorm';

import {
  type CreateNotificationRequest,
  type NotificationRequestRecord,
  type NotificationRequestStore,
} from '../application/notification-request.store';
import {
  NOTIFICATION_REQUEST_STATUS_REQUESTED,
  NotificationRequestEntity,
} from './notification-request.entity';

@Injectable()
export class TypeOrmNotificationRequestStore implements NotificationRequestStore {
  constructor(
    @InjectRepository(NotificationRequestEntity)
    private readonly repository: Repository<NotificationRequestEntity>,
  ) {}

  async create(
    input: CreateNotificationRequest,
  ): Promise<NotificationRequestRecord> {
    const entity = this.repository.create({
      id: randomUUID(),
      orderId: input.orderId,
      shipmentId: input.shipmentId,
      notificationType: input.notificationType,
      status: NOTIFICATION_REQUEST_STATUS_REQUESTED,
    });

    return this.toRecord(await this.repository.save(entity));
  }

  async findById(id: string): Promise<NotificationRequestRecord | null> {
    const entity = await this.repository.findOneBy({ id });

    return entity ? this.toRecord(entity) : null;
  }

  private toRecord(
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
}
