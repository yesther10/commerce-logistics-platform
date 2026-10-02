<?php

namespace App\Console\Commands;

use App\Events\EventPublisher;
use App\Models\OutboxEvent;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use Throwable;

#[Signature('events:publish {--limit=100 : Maximum number of pending events to process} {--loop= : Repeat until interrupted, waiting this many seconds between polls}')]
#[Description('Publish pending outbox events to RabbitMQ.')]
class PublishOutboxEvents extends Command
{
    public function __construct(private readonly EventPublisher $publisher)
    {
        parent::__construct();
    }

    /**
     * Execute the console command.
     */
    public function handle(): int
    {
        $limit = max(1, (int) $this->option('limit'));
        $loop = $this->option('loop');

        do {
            [$published, $failed] = $this->publishPending($limit);
            $this->info("Published {$published} outbox event(s).");

            if ($loop === null || $loop === false) {
                return $failed === 0 ? self::SUCCESS : self::FAILURE;
            }

            sleep(max(1, (int) $loop));
        } while (true);
    }

    /**
     * @return array{int, int}
     */
    private function publishPending(int $limit): array
    {
        $published = 0;
        $failed = 0;

        OutboxEvent::query()
            ->pending()
            ->limit($limit)
            ->get()
            ->each(function (OutboxEvent $event) use (&$failed, &$published): void {
                try {
                    $this->publisher->publish($event->toEnvelope());

                    if ($event->markPublished()) {
                        $published++;
                    }
                } catch (Throwable $exception) {
                    $failed++;
                    $this->error(sprintf('%s: %s', $event->event_id, $exception->getMessage()));
                }
            });

        return [$published, $failed];
    }
}
