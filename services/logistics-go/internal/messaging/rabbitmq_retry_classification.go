package messaging

import (
	"context"
	"errors"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

const (
	retryCountHeader      = "x-logistics-retry-count"
	retryLastErrorHeader  = "x-logistics-retry-last-error"
	retryErrorClassHeader = "x-logistics-retry-error-class"
)

func parseRetryCount(headers amqp.Table) (int, error) {
	value, ok := headers[retryCountHeader]
	if !ok {
		return 0, nil
	}
	var count int
	switch value := value.(type) {
	case int:
		count = value
	case int32:
		count = int(value)
	case int64:
		count = int(value)
	case string:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, errors.New("invalid retry count header")
		}
		count = parsed
	default:
		return 0, errors.New("invalid retry count header")
	}
	if count < 0 || count > RetryQueueCount {
		return 0, errors.New("invalid retry count header")
	}
	return count, nil
}

func classifyShipmentRequested(body []byte) (events.Envelope, string, error) {
	event, err := events.Parse(body)
	if err != nil {
		return events.Envelope{}, "malformed", err
	}
	if _, err := event.ShipmentRequestedOrderID(); err != nil {
		return event, "validation", err
	}
	return event, "", nil
}

func classifyProcessingFailure(err error) string {
	if errors.Is(err, domain.ErrInvalidOrderID) {
		return "validation"
	}
	return "processing"
}

func retryHeaders(retryCount int, errorClass string) amqp.Table {
	errorClass = safeErrorClass(errorClass)
	return amqp.Table{retryCountHeader: int32(retryCount), retryLastErrorHeader: errorClass, retryErrorClassHeader: errorClass}
}

func safeErrorClass(value string) string {
	switch value {
	case "validation", "processing", "retry_topology":
		return value
	default:
		return "unknown"
	}
}

func publishFinalDeadLetter(ctx context.Context, publisher RetryPublisher, body []byte, event events.Envelope, retryCount int, errorClass string) error {
	return publisher.PublishDeadLetter(ctx, RetryMessage{Body: append([]byte(nil), body...), Event: event, Headers: retryHeaders(retryCount, errorClass), RetryCount: retryCount})
}
