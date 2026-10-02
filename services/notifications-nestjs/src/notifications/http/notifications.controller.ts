import {
  Body,
  Controller,
  Get,
  NotFoundException,
  Param,
  ParseUUIDPipe,
  Post,
} from '@nestjs/common';

import { NotificationRequestsService } from '../application/notification-requests.service';
import type { NotificationRequestRecord } from '../application/notification-request.store';
import { CreateNotificationRequestDto } from './dto/create-notification-request.dto';

@Controller('notifications')
export class NotificationsController {
  constructor(
    private readonly notificationRequestsService: NotificationRequestsService,
  ) {}

  @Post()
  async create(@Body() dto: CreateNotificationRequestDto) {
    const notification = await this.notificationRequestsService.create({
      orderId: dto.order_id,
      shipmentId: dto.shipment_id,
      notificationType: dto.notification_type,
    });

    return this.toResponse(notification);
  }

  @Get(':id')
  async findById(@Param('id', new ParseUUIDPipe()) id: string) {
    const notification = await this.notificationRequestsService.findById(id);

    if (!notification) {
      throw new NotFoundException(`Notification request ${id} not found`);
    }

    return this.toResponse(notification);
  }

  private toResponse(notification: NotificationRequestRecord) {
    return {
      id: notification.id,
      order_id: notification.orderId,
      shipment_id: notification.shipmentId,
      notification_type: notification.notificationType,
      status: notification.status,
      created_at: notification.createdAt.toISOString(),
      updated_at: notification.updatedAt.toISOString(),
    };
  }
}
