import { MigrationInterface, QueryRunner, Table } from 'typeorm';

export class CreateNotificationRequests1710000000000 implements MigrationInterface {
  name = 'CreateNotificationRequests1710000000000';

  async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.createTable(
      new Table({
        name: 'notification_requests',
        columns: [
          {
            name: 'id',
            type: 'uuid',
            isPrimary: true,
          },
          {
            name: 'order_id',
            type: 'varchar',
            length: '255',
          },
          {
            name: 'shipment_id',
            type: 'varchar',
            length: '255',
          },
          {
            name: 'notification_type',
            type: 'varchar',
            length: '100',
          },
          {
            name: 'status',
            type: 'varchar',
            length: '30',
            default: "'requested'",
          },
          {
            name: 'created_at',
            type: 'timestamptz',
            default: 'CURRENT_TIMESTAMP',
          },
          {
            name: 'updated_at',
            type: 'timestamptz',
            default: 'CURRENT_TIMESTAMP',
          },
        ],
      }),
      true,
    );
  }

  async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropTable('notification_requests');
  }
}
