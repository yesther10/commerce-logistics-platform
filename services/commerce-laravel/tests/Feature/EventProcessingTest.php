<?php

namespace Tests\Feature;

use App\Events\EventEnvelope;
use App\Events\ProcessOrderCreated;
use App\Models\Order;
use App\Models\OutboxEvent;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class EventProcessingTest extends TestCase
{
    use RefreshDatabase;

    public function test_order_created_approves_payment_and_records_follow_up_events_atomically(): void
    {
        $order = Order::factory()->create();
        $event = new EventEnvelope(
            eventId: '11111111-1111-1111-1111-111111111111',
            eventType: 'OrderCreated',
            eventVersion: 1,
            occurredAt: new \DateTimeImmutable('2026-01-01T12:00:00+00:00'),
            correlationId: '22222222-2222-2222-2222-222222222222',
            causationId: null,
            payload: ['order_id' => $order->id],
        );

        $processed = (new ProcessOrderCreated)->handle($event);

        $this->assertTrue($processed);
        $this->assertSame('payment_approved', $order->refresh()->status);
        $this->assertDatabaseHas('processed_events', [
            'event_id' => '11111111-1111-1111-1111-111111111111',
            'event_type' => 'OrderCreated',
        ]);
        $this->assertDatabaseHas('outbox_events', [
            'event_type' => 'PaymentApproved',
            'correlation_id' => '22222222-2222-2222-2222-222222222222',
            'causation_id' => '11111111-1111-1111-1111-111111111111',
        ]);
        $this->assertDatabaseHas('outbox_events', [
            'event_type' => 'ShipmentRequested',
            'correlation_id' => '22222222-2222-2222-2222-222222222222',
        ]);

        $shipmentRequested = OutboxEvent::query()
            ->where('event_type', 'ShipmentRequested')
            ->firstOrFail();
        $paymentApproved = OutboxEvent::query()
            ->where('event_type', 'PaymentApproved')
            ->firstOrFail();

        $this->assertSame($paymentApproved->event_id, $shipmentRequested->causation_id);
        $this->assertFalse((new ProcessOrderCreated)->handle($event));
        $this->assertDatabaseCount('processed_events', 1);
        $this->assertDatabaseCount('outbox_events', 2);
    }
}
