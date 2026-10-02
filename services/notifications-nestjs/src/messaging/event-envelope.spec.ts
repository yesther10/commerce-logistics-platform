import {
  parseEventEnvelope,
  serializeEventEnvelope,
  SHIPMENT_CREATED_EVENT,
} from './event-envelope';

describe('event envelope', () => {
  it('serializes the published field names and UTC timestamp', () => {
    const envelope = {
      event_id: '11111111-1111-1111-1111-111111111111',
      event_type: SHIPMENT_CREATED_EVENT,
      event_version: 1,
      occurred_at: '2026-01-01T12:00:00Z',
      correlation_id: '22222222-2222-2222-2222-222222222222',
      causation_id: null,
      payload: { order_id: 'order-1' },
    };

    expect(serializeEventEnvelope(envelope).toString()).toBe(
      JSON.stringify(envelope),
    );
    expect(parseEventEnvelope(envelope)).toEqual(envelope);
  });

  it('rejects malformed identifiers and unsupported versions', () => {
    expect(() =>
      parseEventEnvelope({
        event_id: 'event-1',
        event_type: 'ShipmentCreated',
        event_version: 2,
        occurred_at: '2026-01-01T00:00:00Z',
        correlation_id: 'correlation-1',
        causation_id: null,
        payload: {},
      }),
    ).toThrow('event_id and correlation_id must be UUIDs');
  });
});
