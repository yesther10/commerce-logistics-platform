<?php

namespace App\Models;

use App\Events\EventEnvelope;
use Illuminate\Database\Eloquent\Builder;
use Illuminate\Database\Eloquent\Model;

class OutboxEvent extends Model
{
    protected $primaryKey = 'event_id';

    public $incrementing = false;

    protected $keyType = 'string';

    protected $fillable = [
        'event_id',
        'event_type',
        'event_version',
        'occurred_at',
        'correlation_id',
        'causation_id',
        'payload',
        'published_at',
    ];

    protected function casts(): array
    {
        return [
            'event_version' => 'integer',
            'occurred_at' => 'immutable_datetime',
            'payload' => 'array',
            'published_at' => 'immutable_datetime',
        ];
    }

    /**
     * @param  Builder<self>  $query
     * @return Builder<self>
     */
    public function scopePending(Builder $query): Builder
    {
        return $query
            ->whereNull('published_at')
            ->orderBy('occurred_at')
            ->orderBy('event_id');
    }

    public function toEnvelope(): EventEnvelope
    {
        return EventEnvelope::fromArray([
            'event_id' => $this->event_id,
            'event_type' => $this->event_type,
            'event_version' => $this->event_version,
            'occurred_at' => $this->occurred_at->toIso8601String(),
            'correlation_id' => $this->correlation_id,
            'causation_id' => $this->causation_id,
            'payload' => $this->payload,
        ]);
    }

    public function markPublished(): bool
    {
        $publishedAt = now();
        $updated = static::query()
            ->where('event_id', $this->event_id)
            ->whereNull('published_at')
            ->update([
                'published_at' => $publishedAt,
                'updated_at' => $publishedAt,
            ]);

        if ($updated === 1) {
            $this->published_at = $publishedAt;
        }

        return $updated === 1;
    }
}
