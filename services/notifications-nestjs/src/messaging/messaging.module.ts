import { Module } from '@nestjs/common';

import { NotificationsModule } from '../notifications/notifications.module';
import { RabbitMqWorker } from './rabbitmq-worker';

@Module({
  imports: [NotificationsModule],
  providers: [RabbitMqWorker],
  exports: [RabbitMqWorker],
})
export class MessagingModule {}
