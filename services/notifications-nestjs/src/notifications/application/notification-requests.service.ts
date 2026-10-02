import { Inject, Injectable } from '@nestjs/common';

import {
  NOTIFICATION_REQUEST_STORE,
  type CreateNotificationRequest,
  type NotificationRequestRecord,
  type NotificationRequestStore,
} from './notification-request.store';

@Injectable()
export class NotificationRequestsService {
  constructor(
    @Inject(NOTIFICATION_REQUEST_STORE)
    private readonly store: NotificationRequestStore,
  ) {}

  create(input: CreateNotificationRequest): Promise<NotificationRequestRecord> {
    return this.store.create(input);
  }

  findById(id: string): Promise<NotificationRequestRecord | null> {
    return this.store.findById(id);
  }
}
