<?php

namespace App\Actions;

use App\Events\EventEnvelope;
use App\Models\Order;
use App\Models\OrderItem;
use App\Models\OutboxEvent;
use App\Models\Product;
use Illuminate\Support\Facades\DB;
use Illuminate\Validation\ValidationException;

class CreateOrderAction
{
    /**
     * @param  array{items: list<array{product_id: string, quantity: int}>}  $data
     */
    public function handle(array $data): Order
    {
        return DB::transaction(function () use ($data): Order {
            $order = Order::create([
                'status' => 'pending',
                'total' => '0.00',
            ]);

            $items = collect($data['items']);
            $products = Product::query()
                ->whereIn('id', $items->pluck('product_id')->all())
                ->lockForUpdate()
                ->get()
                ->keyBy('id');

            if ($products->count() !== $items->pluck('product_id')->unique()->count()) {
                throw ValidationException::withMessages([
                    'items' => 'One or more selected products are no longer available.',
                ]);
            }

            $totalCents = 0;

            foreach ($items as $item) {
                $product = $products->get($item['product_id']);
                $lineTotalCents = $product->priceInCents() * $item['quantity'];

                OrderItem::create([
                    'order_id' => $order->id,
                    'product_id' => $product->id,
                    'quantity' => $item['quantity'],
                    'unit_price' => $product->price,
                    'line_total' => $this->formatCents($lineTotalCents),
                ]);

                $totalCents += $lineTotalCents;
            }

            $order->update(['total' => $this->formatCents($totalCents)]);

            OutboxEvent::create(EventEnvelope::create(
                eventType: 'OrderCreated',
                payload: ['order_id' => $order->id],
            )->toOutboxAttributes());

            return $order->load('items.product');
        });
    }

    private function formatCents(int $cents): string
    {
        return intdiv($cents, 100).'.'.str_pad((string) ($cents % 100), 2, '0', STR_PAD_LEFT);
    }
}
