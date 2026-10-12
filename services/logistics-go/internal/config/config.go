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
	RabbitMQRetryQueue1     string
	RabbitMQRetryQueue2     string
	RabbitMQRetryQueue3     string
	RabbitMQRetryDelay1     time.Duration
	RabbitMQRetryDelay2     time.Duration
	RabbitMQRetryDelay3     time.Duration
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
	retryDelay1, err := parseDuration("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_1", time.Second)
	if err != nil {
		return Config{}, err
	}
	retryDelay2, err := parseDuration("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_2", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	retryDelay3, err := parseDuration("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_3", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	if err := validateRetryDelays(retryDelay1, retryDelay2, retryDelay3); err != nil {
		return Config{}, err
	}
	retryQueue1 := envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_1", "logistics.shipment.requested.retry.1")
	retryQueue2 := envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_2", "logistics.shipment.requested.retry.2")
	retryQueue3 := envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_3", "logistics.shipment.requested.retry.3")
	if err := validateRetryQueueNames(retryQueue1, retryQueue2, retryQueue3); err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL:             databaseURL,
		HTTPAddr:                httpAddr,
		RabbitMQEnabled:         enabled,
		RabbitMQURL:             envOrDefault("RABBITMQ_URL", "amqp://dev_rabbitmq:dev_rabbitmq_password@127.0.0.1:5673/"),
		RabbitMQQueue:           envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_QUEUE", "logistics.shipment.requested"),
		RabbitMQDeadLetterQueue: envOrDefault("RABBITMQ_SHIPMENT_REQUESTED_DLQ", "logistics.shipment.requested.dlq"),
		RabbitMQRetryQueue1:     retryQueue1,
		RabbitMQRetryQueue2:     retryQueue2,
		RabbitMQRetryQueue3:     retryQueue3,
		RabbitMQRetryDelay1:     retryDelay1,
		RabbitMQRetryDelay2:     retryDelay2,
		RabbitMQRetryDelay3:     retryDelay3,
		RabbitMQConfirmTimeout:  confirmTimeout,
		OutboxPollInterval:      pollInterval,
		OutboxBatchSize:         batchSize,
	}, nil
}

func validateRetryQueueNames(names ...string) error {
	for index, name := range names {
		if name == "" {
			return errors.New("RABBITMQ shipment retry queue " + strconv.Itoa(index+1) + " must not be empty")
		}
	}
	return nil
}

func validateRetryDelays(delays ...time.Duration) error {
	previousMilliseconds := int64(0)
	for index, delay := range delays {
		if delay <= 0 || delay%time.Millisecond != 0 {
			return errors.New("ShipmentRequested retry delays must be positive whole milliseconds")
		}
		milliseconds := delay.Milliseconds()
		if index > 0 && milliseconds <= previousMilliseconds {
			return errors.New("ShipmentRequested retry delays must be strictly increasing at broker millisecond granularity")
		}
		previousMilliseconds = milliseconds
	}
	return nil
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
