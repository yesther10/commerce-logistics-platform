<?php

namespace Tests\Unit;

use App\Events\EventEnvelope;
use DateTimeImmutable;
use PHPUnit\Framework\TestCase;

class EventEnvelopeTest extends TestCase
{
    public function test_it_serializes_the_version_one_envelope_and_routing_key(): void
    {
        $envelope = new EventEnvelope(
            eventId: 'event-1',
            eventType: 'OrderCreated',
            eventVersion: 1,
            occurredAt: new DateTimeImmutable('2026-01-01T12:00:00+00:00'),
            correlationId: 'correlation-1',
            causationId: null,
            payload: ['order_id' => 'order-1'],
        );

        $this->assertSame([
            'event_id' => 'event-1',
            'event_type' => 'OrderCreated',
            'event_version' => 1,
            'occurred_at' => '2026-01-01T12:00:00Z',
            'correlation_id' => 'correlation-1',
            'causation_id' => null,
            'payload' => ['order_id' => 'order-1'],
        ], $envelope->toArray());
        $this->assertSame('commerce.order.created', $envelope->routingKey());
    }
}
