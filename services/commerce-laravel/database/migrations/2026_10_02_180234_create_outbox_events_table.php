<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Run the migrations.
     */
    public function up(): void
    {
        Schema::create('outbox_events', function (Blueprint $table) {
            $table->uuid('event_id')->primary();
            $table->string('event_type');
            $table->unsignedInteger('event_version');
            $table->dateTime('occurred_at');
            $table->uuid('correlation_id');
            $table->uuid('causation_id')->nullable();
            $table->json('payload');
            $table->dateTime('published_at')->nullable();
            $table->timestamps();

            $table->index(['published_at', 'occurred_at']);
            $table->index('event_type');
        });
    }

    /**
     * Reverse the migrations.
     */
    public function down(): void
    {
        Schema::dropIfExists('outbox_events');
    }
};
