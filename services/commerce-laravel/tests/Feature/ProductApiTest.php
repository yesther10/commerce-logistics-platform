<?php

namespace Tests\Feature;

use App\Models\Product;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ProductApiTest extends TestCase
{
    use RefreshDatabase;

    public function test_product_creation_persists_and_returns_a_resource(): void
    {
        $response = $this->postJson('/api/v1/products', [
            'name' => 'Wireless Keyboard',
            'price' => '49.90',
        ]);

        $response
            ->assertCreated()
            ->assertJsonPath('data.name', 'Wireless Keyboard')
            ->assertJsonPath('data.price', '49.90');

        $this->assertDatabaseHas('products', [
            'name' => 'Wireless Keyboard',
            'price' => '49.90',
        ]);
    }

    public function test_product_creation_rejects_an_invalid_payload(): void
    {
        $response = $this->postJson('/api/v1/products', [
            'name' => '',
            'price' => 'free',
        ]);

        $response
            ->assertUnprocessable()
            ->assertJsonValidationErrors(['name', 'price']);

        $this->assertDatabaseCount('products', 0);
    }

    public function test_products_can_be_listed_in_deterministic_order(): void
    {
        $older = Product::factory()->create(['name' => 'Older product']);
        $newer = Product::factory()->create(['name' => 'Newer product']);

        $response = $this->getJson('/api/v1/products');

        $response
            ->assertOk()
            ->assertJsonPath('data.0.id', $newer->id)
            ->assertJsonPath('data.1.id', $older->id);
    }
}
