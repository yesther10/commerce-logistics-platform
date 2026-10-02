import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';

import { NotificationRequestsService } from './application/notification-requests.service';
import { NotificationEventProcessor } from './application/notification-event-processor';
import {
  NOTIFICATION_EVENT_UNIT_OF_WORK,
  type NotificationEventUnitOfWork,
} from './application/notification-workflow';
import {
  NOTIFICATION_REQUEST_STORE,
  type NotificationRequestStore,
} from './application/notification-request.store';
import { NotificationsController } from './http/notifications.controller';
import { NotificationRequestEntity } from './persistence/notification-request.entity';
import { TypeOrmNotificationRequestStore } from './persistence/typeorm-notification-request.store';
import { OutboxEventEntity } from './persistence/outbox-event.entity';
import { ProcessedEventEntity } from './persistence/processed-event.entity';
import { TypeOrmNotificationEventStore } from './persistence/typeorm-notification-event.store';

@Module({
  imports: [
    TypeOrmModule.forFeature([
      NotificationRequestEntity,
      ProcessedEventEntity,
      OutboxEventEntity,
    ]),
  ],
  controllers: [NotificationsController],
  providers: [
    NotificationRequestsService,
    TypeOrmNotificationRequestStore,
    TypeOrmNotificationEventStore,
    NotificationEventProcessor,
    {
      provide: NOTIFICATION_REQUEST_STORE,
      useExisting: TypeOrmNotificationRequestStore,
    },
    {
      provide: NOTIFICATION_EVENT_UNIT_OF_WORK,
      useExisting: TypeOrmNotificationEventStore,
    },
  ],
  exports: [
    NotificationRequestsService,
    NotificationEventProcessor,
    TypeOrmNotificationEventStore,
    NOTIFICATION_EVENT_UNIT_OF_WORK,
  ],
})
export class NotificationsModule {}

export type { NotificationRequestStore };
export type { NotificationEventUnitOfWork };
