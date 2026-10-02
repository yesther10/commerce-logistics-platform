<?php

namespace App\Events;

use DateTimeImmutable;
use DateTimeZone;
use Illuminate\Support\Str;
use InvalidArgumentException;

final readonly class EventEnvelope
{
    private const ROUTING_KEYS = [
        'OrderCreated' => 'commerce.order.created',
        'PaymentApproved' => 'commerce.payment.approved',
        'ShipmentRequested' => 'commerce.shipment.requested',
    ];

    /**
     * @param  array<string, mixed>  $payload
     */
    public function __construct(
        public string $eventId,
        public string $eventType,
        public int $eventVersion,
        public DateTimeImmutable $occurredAt,
        public string $correlationId,
        public ?string $causationId,
        public array $payload,
    ) {}

    /**
     * @param  array<string, mixed>  $payload
     */
    public static function create(
        string $eventType,
        array $payload,
        ?string $correlationId = null,
        ?string $causationId = null,
        int $eventVersion = 1,
    ): self {
        return new self(
            eventId: (string) Str::uuid(),
            eventType: $eventType,
            eventVersion: $eventVersion,
            occurredAt: new DateTimeImmutable('now', new DateTimeZone('UTC')),
            correlationId: $correlationId ?? (string) Str::uuid(),
            causationId: $causationId,
            payload: $payload,
        );
    }

    /**
     * @param  array{event_id: string, event_type: string, event_version: int, occurred_at: string, correlation_id: string, causation_id: ?string, payload: array<string, mixed>}  $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            eventId: $data['event_id'],
            eventType: $data['event_type'],
            eventVersion: $data['event_version'],
            occurredAt: new DateTimeImmutable($data['occurred_at']),
            correlationId: $data['correlation_id'],
            causationId: $data['causation_id'],
            payload: $data['payload'],
        );
    }

    /**
     * @return array{event_id: string, event_type: string, event_version: int, occurred_at: string, correlation_id: string, causation_id: ?string, payload: array<string, mixed>}
     */
    public function toArray(): array
    {
        return [
            'event_id' => $this->eventId,
            'event_type' => $this->eventType,
            'event_version' => $this->eventVersion,
            'occurred_at' => $this->occurredAt->setTimezone(new DateTimeZone('UTC'))->format('Y-m-d\TH:i:s\Z'),
            'correlation_id' => $this->correlationId,
            'causation_id' => $this->causationId,
            'payload' => $this->payload,
        ];
    }

    public function routingKey(): string
    {
        if (! array_key_exists($this->eventType, self::ROUTING_KEYS)) {
            throw new InvalidArgumentException("Unsupported event type [{$this->eventType}].");
        }

        return self::ROUTING_KEYS[$this->eventType];
    }

    /**
     * @return array<string, mixed>
     */
    public function toOutboxAttributes(): array
    {
        return [
            'event_id' => $this->eventId,
            'event_type' => $this->eventType,
            'event_version' => $this->eventVersion,
            'occurred_at' => $this->occurredAt,
            'correlation_id' => $this->correlationId,
            'causation_id' => $this->causationId,
            'payload' => $this->payload,
        ];
    }
}
