import {
  Column,
  CreateDateColumn,
  Entity,
  PrimaryColumn,
  UpdateDateColumn,
  Index,
} from 'typeorm';

export const NOTIFICATION_REQUEST_STATUS_REQUESTED = 'requested' as const;
export const NOTIFICATION_REQUEST_STATUS_DELIVERED = 'delivered' as const;

export type NotificationRequestStatus =
  | typeof NOTIFICATION_REQUEST_STATUS_REQUESTED
  | typeof NOTIFICATION_REQUEST_STATUS_DELIVERED;

@Entity({ name: 'notification_requests' })
@Index(
  'notification_requests_identity_unique',
  ['orderId', 'shipmentId', 'notificationType'],
  { unique: true },
)
export class NotificationRequestEntity {
  @PrimaryColumn({ type: 'uuid' })
  id!: string;

  @Column({ name: 'order_id', type: 'varchar', length: 255 })
  orderId!: string;

  @Column({ name: 'shipment_id', type: 'varchar', length: 255 })
  shipmentId!: string;

  @Column({ name: 'notification_type', type: 'varchar', length: 100 })
  notificationType!: string;

  @Column({
    type: 'varchar',
    length: 30,
    default: NOTIFICATION_REQUEST_STATUS_REQUESTED,
  })
  status!: NotificationRequestStatus;

  @CreateDateColumn({ name: 'created_at', type: 'timestamptz' })
  createdAt!: Date;

  @UpdateDateColumn({ name: 'updated_at', type: 'timestamptz' })
  updatedAt!: Date;
}
