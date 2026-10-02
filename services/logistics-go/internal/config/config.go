package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL             string
	HTTPAddr                string
	RabbitMQEnabled         bool
	RabbitMQURL             string
	RabbitMQQueue           string
	RabbitMQDeadLetterQueue string
	RabbitMQConfirmTimeout  time.Duration
	OutboxPollInterval      time.Duration
	OutboxBatchSize         int
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	enabled, err := parseBool("RABBITMQ_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	confirmTimeout, err := parseDuration("RABBITMQ_CONFIRM_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	pollInterval, err := parseDuration("OUTBOX_POLL_INTERVAL", time.Second)
	if err != nil {
		return Config{}, err
	}
	batchSize, err := parseInt("OUTBOX_BATCH_SIZE", 100)
	if err != nil || batchSize < 1 {
		return Config{}, errors.New("OUTBOX_BATCH_SIZE must be a positive integer")
	}

	return Config{
		DatabaseURL:             databaseURL,
		HTTPAddr:                httpAddr,
		RabbitMQEnabled:         enabled,
		RabbitMQURL:             envOrDefault("RABBITMQ_URL", "amqp://dev_rabbitmq:dev_rabbitmq_password@127.0.0.1:5673/"),
		RabbitMQQueue:           envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_QUEUE", "logistics.shipment.requested"),
		RabbitMQDeadLetterQueue: envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_DLQ", "logistics.shipment.requested.dlq"),
		RabbitMQConfirmTimeout:  confirmTimeout,
		OutboxPollInterval:      pollInterval,
		OutboxBatchSize:         batchSize,
	}, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func parseBool(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New(name + " must be a boolean")
	}
	return parsed, nil
}

func parseDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New(name + " must be a positive duration")
	}
	return parsed, nil
}

func parseInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}
