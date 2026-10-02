import { Test } from '@nestjs/testing';

import {
  NOTIFICATION_REQUEST_STORE,
  type NotificationRequestStore,
} from './notification-request.store';
import { NotificationRequestsService } from './notification-requests.service';

describe('NotificationRequestsService', () => {
  let service: NotificationRequestsService;
  let store: jest.Mocked<NotificationRequestStore>;

  beforeEach(async () => {
    store = {
      create: jest.fn(),
      findById: jest.fn(),
    };

    const moduleRef = await Test.createTestingModule({
      providers: [
        NotificationRequestsService,
        { provide: NOTIFICATION_REQUEST_STORE, useValue: store },
      ],
    }).compile();

    service = moduleRef.get(NotificationRequestsService);
  });

  it('creates a notification request through the injected store', async () => {
    const input = {
      orderId: 'order-123',
      shipmentId: 'shipment-123',
      notificationType: 'shipment_created',
    };
    const created = {
      ...input,
      id: '7b3f5a08-84fd-4576-a16b-e4e4ec2d8da9',
      status: 'requested' as const,
      createdAt: new Date('2026-01-01T00:00:00.000Z'),
      updatedAt: new Date('2026-01-01T00:00:00.000Z'),
    };
    store.create.mockResolvedValue(created);

    await expect(service.create(input)).resolves.toEqual(created);
    expect(store.create).toHaveBeenCalledWith(input);
  });

  it('returns the stored notification request by id', async () => {
    const stored = {
      id: '7b3f5a08-84fd-4576-a16b-e4e4ec2d8da9',
      orderId: 'order-123',
      shipmentId: 'shipment-123',
      notificationType: 'shipment_created',
      status: 'requested' as const,
      createdAt: new Date('2026-01-01T00:00:00.000Z'),
      updatedAt: new Date('2026-01-01T00:00:00.000Z'),
    };
    store.findById.mockResolvedValue(stored);

    await expect(service.findById(stored.id)).resolves.toEqual(stored);
    expect(store.findById).toHaveBeenCalledWith(stored.id);
  });
});
