import { MigrationInterface, QueryRunner, Table, TableIndex } from 'typeorm';

export class CreateEventingTables1720000000000 implements MigrationInterface {
  name = 'CreateEventingTables1720000000000';

  async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.createTable(
      new Table({
        name: 'processed_events',
        columns: [
          { name: 'event_id', type: 'uuid', isPrimary: true },
          { name: 'event_type', type: 'varchar', length: '100' },
          { name: 'processed_at', type: 'timestamptz' },
        ],
      }),
      true,
    );
    await queryRunner.createTable(
      new Table({
        name: 'outbox_events',
        columns: [
          { name: 'event_id', type: 'uuid', isPrimary: true },
          { name: 'event_type', type: 'varchar', length: '100' },
          { name: 'event_version', type: 'integer' },
          { name: 'occurred_at', type: 'timestamptz' },
          { name: 'correlation_id', type: 'uuid' },
          { name: 'causation_id', type: 'uuid', isNullable: true },
          { name: 'payload', type: 'jsonb' },
          { name: 'published_at', type: 'timestamptz', isNullable: true },
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
    await queryRunner.createIndex(
      'notification_requests',
      new TableIndex({
        name: 'notification_requests_identity_unique',
        columnNames: ['order_id', 'shipment_id', 'notification_type'],
        isUnique: true,
      }),
    );
  }

  async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.dropIndex(
      'notification_requests',
      'notification_requests_identity_unique',
    );
    await queryRunner.dropTable('outbox_events', true);
    await queryRunner.dropTable('processed_events', true);
  }
}
