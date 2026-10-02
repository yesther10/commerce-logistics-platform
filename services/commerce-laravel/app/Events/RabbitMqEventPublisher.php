<?php

namespace App\Events;

use PhpAmqpLib\Connection\AMQPStreamConnection;
use PhpAmqpLib\Message\AMQPMessage;
use RuntimeException;

final class RabbitMqEventPublisher implements EventPublisher
{
    public function publish(EventEnvelope $event): void
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

        try {
            RabbitMqTopology::declarePublisher($channel);
            $channel->confirm_select();

            $message = new AMQPMessage(
                json_encode($event->toArray(), JSON_THROW_ON_ERROR),
                [
                    'content_type' => 'application/json',
                    'delivery_mode' => AMQPMessage::DELIVERY_MODE_PERSISTENT,
                    'message_id' => $event->eventId,
                    'type' => $event->eventType,
                    'correlation_id' => $event->correlationId,
                ],
            );

            $channel->basic_publish($message, RabbitMqTopology::EXCHANGE, $event->routingKey());
            $channel->wait_for_pending_acks_returns((float) config('services.rabbitmq.publisher_confirm_timeout'));
        } catch (\Throwable $exception) {
            throw new RuntimeException('RabbitMQ did not confirm event publication.', previous: $exception);
        } finally {
            $channel->close();
            $connection->close();
        }
    }
}
