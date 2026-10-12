package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load logistics worker configuration", "service", "logistics-go", "component", "worker", "error_class", "configuration")
		os.Exit(1)
	}
	if !cfg.RabbitMQEnabled {
		logger.Error("RabbitMQ is disabled for the logistics worker", "service", "logistics-go", "component", "worker", "error_class", "configuration")
		os.Exit(1)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to PostgreSQL", "service", "logistics-go", "component", "worker", "error_class", "database")
		os.Exit(1)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("connect to PostgreSQL", "service", "logistics-go", "component", "worker", "error_class", "database")
		os.Exit(1)
	}
	if err := storage.ApplyMigrations(ctx, db); err != nil {
		logger.Error("apply migrations", "service", "logistics-go", "component", "worker", "error_class", "database")
		os.Exit(1)
	}

	connection, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Error("connect to RabbitMQ", "service", "logistics-go", "component", "worker", "error_class", "broker")
		os.Exit(1)
	}
	defer connection.Close()
	consumerChannel, err := connection.Channel()
	if err != nil {
		logger.Error("open RabbitMQ consumer channel", "service", "logistics-go", "component", "worker", "error_class", "broker")
		os.Exit(1)
	}
	defer consumerChannel.Close()
	publisherChannel, err := connection.Channel()
	if err != nil {
		logger.Error("open RabbitMQ publisher channel", "service", "logistics-go", "component", "worker", "error_class", "broker")
		os.Exit(1)
	}
	defer publisherChannel.Close()

	repository := storage.NewPostgresRepository(db)
	processor := application.NewShipmentRequestedHandler(repository)
	publisher, err := messaging.NewPublisher(publisherChannel, cfg.RabbitMQConfirmTimeout)
	if err != nil {
		logger.Error("enable RabbitMQ publisher confirms", "service", "logistics-go", "component", "worker", "error_class", "broker")
		os.Exit(1)
	}
	topology := messaging.TopologyConfig{
		Queue:           cfg.RabbitMQQueue,
		DeadLetterQueue: cfg.RabbitMQDeadLetterQueue,
		RetryQueues: []messaging.RetryQueueConfig{
			{Queue: cfg.RabbitMQRetryQueue1, Delay: cfg.RabbitMQRetryDelay1},
			{Queue: cfg.RabbitMQRetryQueue2, Delay: cfg.RabbitMQRetryDelay2},
			{Queue: cfg.RabbitMQRetryQueue3, Delay: cfg.RabbitMQRetryDelay3},
		},
	}
	go publishOutbox(ctx, repository, publisher, cfg.OutboxPollInterval, cfg.OutboxBatchSize, logger)

	logger.Info("logistics worker consuming ShipmentRequested", "service", "logistics-go", "component", "worker", "queue", cfg.RabbitMQQueue, "event_id", "", "event_type", "ShipmentRequested", "correlation_id", "", "retry_count", 0, "outcome", "started")
	if err := messaging.ConsumeShipmentRequested(ctx, consumerChannel, topology, processor, messaging.WithRetryPublisher(publisher), messaging.WithAttemptObserver(messaging.NewSlogAttemptObserver(logger))); err != nil && ctx.Err() == nil {
		logger.Error("logistics worker stopped", "service", "logistics-go", "component", "worker", "queue", cfg.RabbitMQQueue, "event_id", "", "event_type", "ShipmentRequested", "correlation_id", "", "retry_count", 0, "outcome", "stopped", "error_class", "consumer")
		os.Exit(1)
	}
}

func publishOutbox(ctx context.Context, store application.OutboxStore, publisher application.EventPublisher, interval time.Duration, batchSize int, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := application.PublishPending(ctx, store, publisher, batchSize); err != nil && ctx.Err() == nil {
			logger.Error("publish logistics outbox", "service", "logistics-go", "component", "outbox_publisher", "queue", "", "event_id", "", "event_type", "", "correlation_id", "", "retry_count", 0, "outcome", "failed", "error_class", "publication")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
