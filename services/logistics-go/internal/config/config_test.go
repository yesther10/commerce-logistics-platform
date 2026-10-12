package config

import (
	"testing"
	"time"
)

func TestLoadIncludesRetryTopologyAndDefaults(t *testing.T) {
	setRequiredEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RabbitMQRetryQueue1 != "logistics.shipment.requested.retry.1" || cfg.RabbitMQRetryQueue2 != "logistics.shipment.requested.retry.2" || cfg.RabbitMQRetryQueue3 != "logistics.shipment.requested.retry.3" {
		t.Fatalf("retry queues = %q, %q, %q", cfg.RabbitMQRetryQueue1, cfg.RabbitMQRetryQueue2, cfg.RabbitMQRetryQueue3)
	}
	if cfg.RabbitMQRetryDelay1 != time.Second || cfg.RabbitMQRetryDelay2 != 5*time.Second || cfg.RabbitMQRetryDelay3 != 15*time.Second {
		t.Fatalf("retry delays = %s, %s, %s", cfg.RabbitMQRetryDelay1, cfg.RabbitMQRetryDelay2, cfg.RabbitMQRetryDelay3)
	}
}

func TestLoadRejectsRetryDelaysThatAreNotStrictAtBrokerMillisecondGranularity(t *testing.T) {
	cases := []struct {
		name   string
		delay1 string
		delay2 string
		delay3 string
	}{
		{name: "fractional millisecond", delay1: "1500us", delay2: "2ms", delay3: "3ms"},
		{name: "equal broker milliseconds", delay1: "1ms", delay2: "1000000ns", delay3: "2ms"},
		{name: "not increasing", delay1: "1ms", delay2: "1ms", delay3: "2ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_1", tc.delay1)
			t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_2", tc.delay2)
			t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_3", tc.delay3)
			if _, err := Load(); err == nil {
				t.Fatal("Load() accepted invalid retry delays")
			}
		})
	}
}

func TestLoadUsesConfiguredRetryTopologyValues(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_1", "custom.retry.one")
	t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_2", "custom.retry.two")
	t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_3", "custom.retry.three")
	t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_1", "2ms")
	t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_2", "20ms")
	t.Setenv("RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_3", "200ms")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RabbitMQRetryQueue1 != "custom.retry.one" || cfg.RabbitMQRetryQueue2 != "custom.retry.two" || cfg.RabbitMQRetryQueue3 != "custom.retry.three" {
		t.Fatalf("retry queues = %q, %q, %q", cfg.RabbitMQRetryQueue1, cfg.RabbitMQRetryQueue2, cfg.RabbitMQRetryQueue3)
	}
	if cfg.RabbitMQRetryDelay1 != 2*time.Millisecond || cfg.RabbitMQRetryDelay2 != 20*time.Millisecond || cfg.RabbitMQRetryDelay3 != 200*time.Millisecond {
		t.Fatalf("retry delays = %s, %s, %s", cfg.RabbitMQRetryDelay1, cfg.RabbitMQRetryDelay2, cfg.RabbitMQRetryDelay3)
	}
}

func TestValidateRetryQueueNamesRejectsAnEmptyName(t *testing.T) {
	if err := validateRetryQueueNames("retry.one", "", "retry.three"); err == nil {
		t.Fatal("validateRetryQueueNames() accepted an empty name")
	}
}

func TestValidateRetryQueueNamesAcceptsConfiguredNames(t *testing.T) {
	if err := validateRetryQueueNames("retry.one", "retry.two", "retry.three"); err != nil {
		t.Fatal(err)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost/logistics")
	for _, name := range []string{
		"HTTP_ADDR",
		"RABBITMQ_ENABLED",
		"RABBITMQ_URL",
		"RABBITMQ_SHIPMENT_REQUESTED_QUEUE",
		"RABBITMQ_SHIPMENT_REQUESTED_DLQ",
		"RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_1",
		"RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_2",
		"RABBITMQ_SHIPMENT_REQUESTED_RETRY_QUEUE_3",
		"RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_1",
		"RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_2",
		"RABBITMQ_SHIPMENT_REQUESTED_RETRY_DELAY_3",
		"RABBITMQ_CONFIRM_TIMEOUT",
		"OUTBOX_POLL_INTERVAL",
		"OUTBOX_BATCH_SIZE",
	} {
		t.Setenv(name, "")
	}
}
