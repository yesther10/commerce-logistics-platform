import type { NotificationRequestStatus } from '../persistence/notification-request.entity';

export const NOTIFICATION_REQUEST_STORE = Symbol('NOTIFICATION_REQUEST_STORE');

export interface CreateNotificationRequest {
  orderId: string;
  shipmentId: string;
  notificationType: string;
}

export interface NotificationRequestRecord extends CreateNotificationRequest {
  id: string;
  status: NotificationRequestStatus;
  createdAt: Date;
  updatedAt: Date;
}

export interface NotificationRequestStore {
  create(input: CreateNotificationRequest): Promise<NotificationRequestRecord>;
  findById(id: string): Promise<NotificationRequestRecord | null>;
}
