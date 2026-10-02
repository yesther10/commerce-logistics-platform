<?php

namespace App\Events;

use App\Models\Order;
use App\Models\OutboxEvent;
use App\Models\ProcessedEvent;
use Illuminate\Support\Facades\DB;
use InvalidArgumentException;

final class ProcessOrderCreated
{
    public function handle(EventEnvelope $event): bool
    {
        if ($event->eventType !== 'OrderCreated' || $event->eventVersion !== 1) {
            throw new InvalidArgumentException('Only OrderCreated version 1 events are supported.');
        }

        $orderId = $event->payload['order_id'] ?? null;

        if (! is_string($orderId)) {
            throw new InvalidArgumentException('OrderCreated payload must include a string order_id.');
        }

        return DB::transaction(function () use ($event, $orderId): bool {
            $timestamp = now();
            $wasInserted = ProcessedEvent::query()->insertOrIgnore([
                'event_id' => $event->eventId,
                'event_type' => $event->eventType,
                'processed_at' => $timestamp,
                'created_at' => $timestamp,
                'updated_at' => $timestamp,
            ]);

            if ($wasInserted === 0) {
                return false;
            }

            $order = Order::query()->lockForUpdate()->findOrFail($orderId);
            $order->update(['status' => 'payment_approved']);

            $paymentApproved = EventEnvelope::create(
                eventType: 'PaymentApproved',
                payload: [
                    'order_id' => $order->id,
                    'payment_status' => 'approved',
                    'simulated' => true,
                ],
                correlationId: $event->correlationId,
                causationId: $event->eventId,
            );
            $shipmentRequested = EventEnvelope::create(
                eventType: 'ShipmentRequested',
                payload: ['order_id' => $order->id],
                correlationId: $event->correlationId,
                causationId: $paymentApproved->eventId,
            );

            OutboxEvent::create($paymentApproved->toOutboxAttributes());
            OutboxEvent::create($shipmentRequested->toOutboxAttributes());

            return true;
        });
    }
}
