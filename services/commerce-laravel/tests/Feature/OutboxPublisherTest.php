<?php

namespace Tests\Feature;

use App\Events\EventEnvelope;
use App\Events\EventPublisher;
use App\Models\OutboxEvent;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Mockery;
use RuntimeException;
use Tests\TestCase;

class OutboxPublisherTest extends TestCase
{
    use RefreshDatabase;

    public function test_confirmed_publication_marks_the_outbox_event_as_published(): void
    {
        $event = EventEnvelope::create('OrderCreated', ['order_id' => 'order-1']);
        OutboxEvent::create($event->toOutboxAttributes());
        $publisher = Mockery::mock(EventPublisher::class);
        $publisher->shouldReceive('publish')->once()->withArgs(function (EventEnvelope $published) use ($event): bool {
            return $published->eventId === $event->eventId && $published->eventType === 'OrderCreated';
        });
        $this->app->instance(EventPublisher::class, $publisher);

        $this->artisan('events:publish')
            ->assertExitCode(0);

        $this->assertNotNull(OutboxEvent::query()->findOrFail($event->eventId)->published_at);
    }

    public function test_a_failed_publication_leaves_the_event_pending_for_at_least_once_retry(): void
    {
        $event = EventEnvelope::create('OrderCreated', ['order_id' => 'order-1']);
        OutboxEvent::create($event->toOutboxAttributes());
        $publisher = Mockery::mock(EventPublisher::class);
        $publisher->shouldReceive('publish')->once()->andThrow(new RuntimeException('broker unavailable'));
        $this->app->instance(EventPublisher::class, $publisher);

        $this->artisan('events:publish')
            ->assertExitCode(1);

        $this->assertNull(OutboxEvent::query()->findOrFail($event->eventId)->published_at);
    }

    public function test_mark_published_is_idempotent(): void
    {
        $event = EventEnvelope::create('OrderCreated', ['order_id' => 'order-1']);
        $outboxEvent = OutboxEvent::create($event->toOutboxAttributes());

        $this->assertTrue($outboxEvent->markPublished());
        $this->assertFalse($outboxEvent->markPublished());
        $this->assertDatabaseCount('outbox_events', 1);
    }
}
