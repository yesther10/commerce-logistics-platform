<?php

namespace App\Events;

use PhpAmqpLib\Channel\AMQPChannel;

final class RabbitMqTopology
{
    public const EXCHANGE = 'platform.events';

    public const DEAD_LETTER_EXCHANGE = 'platform.events.dlx';

    public static function declarePublisher(AMQPChannel $channel): void
    {
        $channel->exchange_declare(self::EXCHANGE, 'topic', false, true, false);
    }

    public static function declareOrderCreatedConsumer(AMQPChannel $channel): void
    {
        self::declarePublisher($channel);
        $channel->exchange_declare(self::DEAD_LETTER_EXCHANGE, 'topic', false, true, false);

        $queue = (string) config('services.rabbitmq.order_created_queue');
        $deadLetterQueue = (string) config('services.rabbitmq.order_created_dead_letter_queue');
        $routingKey = 'commerce.order.created';

        $channel->queue_declare($queue, false, true, false, false, false, [
            'x-dead-letter-exchange' => ['S', self::DEAD_LETTER_EXCHANGE],
            'x-dead-letter-routing-key' => ['S', $routingKey],
        ]);
        $channel->queue_bind($queue, self::EXCHANGE, $routingKey);

        $channel->queue_declare($deadLetterQueue, false, true, false, false);
        $channel->queue_bind($deadLetterQueue, self::DEAD_LETTER_EXCHANGE, $routingKey);
    }
}
