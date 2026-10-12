package messaging

import (
	"context"
	"log/slog"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type SlogAttemptObserver struct{ logger *slog.Logger }

func NewSlogAttemptObserver(logger *slog.Logger) *SlogAttemptObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogAttemptObserver{logger: logger}
}
func WithAttemptObserver(observer AttemptObserver) ConsumerOption {
	return func(options *consumerOptions) {
		if observer != nil {
			options.observer = observer
		}
	}
}

func (o *SlogAttemptObserver) Observe(ctx context.Context, attempt Attempt) {
	attrs := []any{"service", "logistics-go", "component", "shipment_requested_consumer", "queue", safeQueueName(attempt.Queue), "event_id", safeUUID(attempt.Event.EventID), "event_type", safeEventType(attempt.Event.EventType), "correlation_id", safeUUID(attempt.Event.CorrelationID), "retry_count", attempt.RetryCount, "outcome", attempt.Outcome}
	if attempt.ErrorClass != "" {
		attrs = append(attrs, "error_class", attempt.ErrorClass)
	}
	o.logger.InfoContext(ctx, "ShipmentRequested processing attempt", attrs...)
}

func safeQueueName(value string) string {
	if len(value) == 0 || len(value) > 128 {
		return ""
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '.' && character != '_' && character != '-' {
			return ""
		}
	}
	return value
}
func safeEventType(value string) string {
	if value == events.ShipmentRequested {
		return value
	}
	return ""
}
func safeUUID(value string) string {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return ""
	}
	for index, character := range value {
		if character == '-' {
			continue
		}
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return ""
		}
		if index > 35 {
			return ""
		}
	}
	return value
}
