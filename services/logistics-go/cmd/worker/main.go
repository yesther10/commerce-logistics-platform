package main

import (
	"context"
	"database/sql"
	"log"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/application"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/config"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/messaging"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.RabbitMQEnabled {
		log.Fatal("RABBITMQ_ENABLED must be true to run the logistics worker")
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := storage.ApplyMigrations(ctx, db); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	connection, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("connect to RabbitMQ: %v", err)
	}
	defer connection.Close()
	consumerChannel, err := connection.Channel()
	if err != nil {
		log.Fatalf("open RabbitMQ consumer channel: %v", err)
	}
	defer consumerChannel.Close()
	publisherChannel, err := connection.Channel()
	if err != nil {
		log.Fatalf("open RabbitMQ publisher channel: %v", err)
	}
	defer publisherChannel.Close()

	repository := storage.NewPostgresRepository(db)
	processor := application.NewShipmentRequestedHandler(repository)
	publisher, err := messaging.NewPublisher(publisherChannel, cfg.RabbitMQConfirmTimeout)
	if err != nil {
		log.Fatalf("enable RabbitMQ publisher confirms: %v", err)
	}
	topology := messaging.TopologyConfig{Queue: cfg.RabbitMQQueue, DeadLetterQueue: cfg.RabbitMQDeadLetterQueue}
	go publishOutbox(ctx, repository, publisher, cfg.OutboxPollInterval, cfg.OutboxBatchSize)

	log.Printf("logistics worker consuming %s", cfg.RabbitMQQueue)
	if err := messaging.ConsumeShipmentRequested(ctx, consumerChannel, topology, processor); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func publishOutbox(ctx context.Context, store application.OutboxStore, publisher application.EventPublisher, interval time.Duration, batchSize int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := application.PublishPending(ctx, store, publisher, batchSize); err != nil && ctx.Err() == nil {
			log.Printf("publish logistics outbox: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
