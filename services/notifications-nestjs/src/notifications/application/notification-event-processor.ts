import { Inject, Injectable } from '@nestjs/common';
import { randomUUID } from 'node:crypto';

import type { EventEnvelope } from '../../messaging/event-envelope';
import {
  NOTIFICATION_EVENT_UNIT_OF_WORK,
  NotificationWorkflow,
  type NotificationEventUnitOfWork,
} from './notification-workflow';

@Injectable()
export class NotificationEventProcessor {
  private readonly workflow = new NotificationWorkflow(
    randomUUID,
    () => new Date(),
  );

  constructor(
    @Inject(NOTIFICATION_EVENT_UNIT_OF_WORK)
    private readonly unitOfWork: NotificationEventUnitOfWork,
  ) {}

  processShipmentCreated(event: EventEnvelope) {
    return this.workflow.processShipmentCreated(this.unitOfWork, event);
  }

  processNotificationRequested(event: EventEnvelope) {
    return this.workflow.processNotificationRequested(this.unitOfWork, event);
  }
}
