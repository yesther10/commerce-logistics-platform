import type { INestApplication } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import * as request from 'supertest';

import { configureApp } from '../src/configure-app';
import { HealthController } from '../src/health/health.controller';
import {
  NOTIFICATION_REQUEST_STORE,
  type NotificationRequestRecord,
  type NotificationRequestStore,
} from '../src/notifications/application/notification-request.store';
import { NotificationRequestsService } from '../src/notifications/application/notification-requests.service';
import { NotificationsController } from '../src/notifications/http/notifications.controller';

describe('Notifications HTTP API', () => {
  let app: INestApplication;
  let store: InMemoryNotificationRequestStore;

  beforeAll(async () => {
    store = new InMemoryNotificationRequestStore();

    const moduleRef = await Test.createTestingModule({
      controllers: [HealthController, NotificationsController],
      providers: [
        NotificationRequestsService,
        { provide: NOTIFICATION_REQUEST_STORE, useValue: store },
      ],
    }).compile();

    app = moduleRef.createNestApplication();
    configureApp(app);
    await app.init();
  });

  afterAll(async () => {
    await app.close();
  });

  it('returns a healthy response', async () => {
    await request(app.getHttpServer())
      .get('/healthz')
      .expect(200)
      .expect({ status: 'ok' });
  });

  it('validates, creates, and reads a notification request', async () => {
    const createResponse = await request(app.getHttpServer())
      .post('/notifications')
      .send({
        order_id: 'order-123',
        shipment_id: 'shipment-123',
        notification_type: 'shipment_created',
      })
      .expect(201);

    expect(createResponse.body).toMatchObject({
      order_id: 'order-123',
      shipment_id: 'shipment-123',
      notification_type: 'shipment_created',
      status: 'requested',
    });

    await request(app.getHttpServer())
      .get(`/notifications/${createResponse.body.id}`)
      .expect(200)
      .expect(createResponse.body);
  });

  it('rejects invalid input and unknown notification requests', async () => {
    await request(app.getHttpServer())
      .post('/notifications')
      .send({ order_id: '', shipment_id: 'shipment-123' })
      .expect(400);

    await request(app.getHttpServer())
      .get('/notifications/6aaf8e1a-fd83-4d50-9db5-e835c8c0e50c')
      .expect(404)
      .expect({
        statusCode: 404,
        error: 'Not Found',
        message:
          'Notification request 6aaf8e1a-fd83-4d50-9db5-e835c8c0e50c not found',
      });
  });
});

class InMemoryNotificationRequestStore implements NotificationRequestStore {
  private readonly records = new Map<string, NotificationRequestRecord>();

  async create(input: {
    orderId: string;
    shipmentId: string;
    notificationType: string;
  }): Promise<NotificationRequestRecord> {
    const now = new Date('2026-01-01T00:00:00.000Z');
    const record: NotificationRequestRecord = {
      ...input,
      id: '7b3f5a08-84fd-4576-a16b-e4e4ec2d8da9',
      status: 'requested',
      createdAt: now,
      updatedAt: now,
    };

    this.records.set(record.id, record);
    return record;
  }

  async findById(id: string): Promise<NotificationRequestRecord | null> {
    return this.records.get(id) ?? null;
  }
}
