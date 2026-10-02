<?php

namespace App\Events;

interface EventPublisher
{
    public function publish(EventEnvelope $event): void;
}
