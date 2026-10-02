<?php

namespace App\Console\Commands;

use App\Events\EventEnvelope;
use App\Events\ProcessOrderCreated;
use App\Events\RabbitMqTopology;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use PhpAmqpLib\Connection\AMQPStreamConnection;
use PhpAmqpLib\Message\AMQPMessage;
use Throwable;

#[Signature('events:consume-order-created {--once : Process one message and exit}')]
#[Description('Consume OrderCreated events and simulate payment approval.')]
class ConsumeOrderCreated extends Command
{
    public function __construct(private readonly ProcessOrderCreated $processor)
    {
        parent::__construct();
    }

    /**
     * Execute the console command.
     */
    public function handle(): int
    {
        $connection = new AMQPStreamConnection(
            (string) config('services.rabbitmq.host'),
            (int) config('services.rabbitmq.port'),
            (string) config('services.rabbitmq.user'),
            (string) config('services.rabbitmq.password'),
            (string) config('services.rabbitmq.vhost'),
            false,
            'AMQPLAIN',
            null,
            'en_US',
            (float) config('services.rabbitmq.connection_timeout'),
            (float) config('services.rabbitmq.read_write_timeout'),
            null,
            false,
            (int) config('services.rabbitmq.heartbeat'),
        );
        $channel = $connection->channel();
        $processed = 0;

        try {
            RabbitMqTopology::declareOrderCreatedConsumer($channel);
            $channel->basic_qos(0, 1, false);

            $channel->basic_consume(
                (string) config('services.rabbitmq.order_created_queue'),
                '',
                false,
                false,
                false,
                false,
                function (AMQPMessage $message) use ($channel, &$processed): void {
                    try {
                        $payload = json_decode($message->getBody(), true, 512, JSON_THROW_ON_ERROR);
                        $wasProcessed = $this->processor->handle(EventEnvelope::fromArray($payload));
                        $message->ack();
                        $processed++;

                        $this->line($wasProcessed ? 'Processed OrderCreated.' : 'Acknowledged duplicate OrderCreated.');

                        if ($this->option('once')) {
                            $channel->basic_cancel($message->getConsumerTag());
                        }
                    } catch (Throwable $exception) {
                        $this->error($exception->getMessage());
                        $message->nack(false, false);
                    }
                },
            );

            while ($channel->is_consuming() && (! $this->option('once') || $processed === 0)) {
                $channel->wait();
            }
        } finally {
            $channel->close();
            $connection->close();
        }

        return self::SUCCESS;
    }
}
