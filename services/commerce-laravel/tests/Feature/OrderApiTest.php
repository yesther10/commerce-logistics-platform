<?php

namespace Tests\Feature;

use App\Models\OrderItem;
use App\Models\Product;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Str;
use RuntimeException;
use Tests\TestCase;

class OrderApiTest extends TestCase
{
    use RefreshDatabase;

    public function test_order_creation_calculates_the_server_side_total_and_persists_items(): void
    {
        $keyboard = Product::factory()->create(['price' => '12.50']);
        $mouse = Product::factory()->create(['price' => '3.10']);

        $response = $this->postJson('/api/v1/orders', [
            'total' => '0.01',
            'items' => [
                ['product_id' => $keyboard->id, 'quantity' => 2],
                ['product_id' => $mouse->id, 'quantity' => 3],
            ],
        ]);

        $response
            ->assertCreated()
            ->assertJsonPath('data.status', 'pending')
            ->assertJsonPath('data.total', '34.30')
            ->assertJsonCount(2, 'data.items');

        $this->assertDatabaseHas('orders', [
            'total' => '34.30',
            'status' => 'pending',
        ]);
        $this->assertDatabaseHas('order_items', [
            'product_id' => $keyboard->id,
            'quantity' => 2,
            'unit_price' => '12.50',
            'line_total' => '25.00',
        ]);
        $this->assertDatabaseHas('order_items', [
            'product_id' => $mouse->id,
            'quantity' => 3,
            'unit_price' => '3.10',
            'line_total' => '9.30',
        ]);
        $this->assertDatabaseHas('outbox_events', [
            'event_type' => 'OrderCreated',
            'causation_id' => null,
            'payload' => json_encode(['order_id' => $response->json('data.id')]),
        ]);
    }

    public function test_order_creation_rejects_invalid_items_without_persisting_an_order(): void
    {
        $response = $this->postJson('/api/v1/orders', [
            'items' => [
                ['product_id' => (string) Str::uuid(), 'quantity' => 0],
            ],
        ]);

        $response
            ->assertUnprocessable()
            ->assertJsonValidationErrors(['items.0.product_id', 'items.0.quantity']);

        $this->assertDatabaseCount('orders', 0);
        $this->assertDatabaseCount('order_items', 0);
    }

    public function test_an_order_can_be_retrieved_with_its_items(): void
    {
        $product = Product::factory()->create(['price' => '8.25']);

        $creation = $this->postJson('/api/v1/orders', [
            'items' => [
                ['product_id' => $product->id, 'quantity' => 2],
            ],
        ])->assertCreated();

        $orderId = $creation->json('data.id');

        $this->getJson("/api/v1/orders/{$orderId}")
            ->assertOk()
            ->assertJsonPath('data.id', $orderId)
            ->assertJsonPath('data.items.0.product.id', $product->id)
            ->assertJsonPath('data.items.0.line_total', '16.50');
    }

    public function test_order_creation_rolls_back_when_item_persistence_fails(): void
    {
        $product = Product::factory()->create(['price' => '10.00']);

        OrderItem::creating(function (): void {
            throw new RuntimeException('simulated item persistence failure');
        });

        try {
            $this->withoutExceptionHandling()->postJson('/api/v1/orders', [
                'items' => [
                    ['product_id' => $product->id, 'quantity' => 1],
                ],
            ]);

            $this->fail('The simulated persistence failure was not raised.');
        } catch (RuntimeException $exception) {
            $this->assertSame('simulated item persistence failure', $exception->getMessage());
        } finally {
            OrderItem::flushEventListeners();
        }

        $this->assertDatabaseCount('orders', 0);
        $this->assertDatabaseCount('order_items', 0);
        $this->assertDatabaseCount('outbox_events', 0);
        $this->assertDatabaseMissing('orders', ['status' => 'pending']);
    }
}
