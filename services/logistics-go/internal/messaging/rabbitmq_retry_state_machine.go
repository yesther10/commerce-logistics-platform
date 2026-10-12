package messaging

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/application"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type deliveryMessage struct {
	Body                                []byte
	Headers                             amqp.Table
	MessageID, EventType, CorrelationID string
}

type deliveryActions interface {
	Ack(bool) error
	Reject(bool) error
}

type ShipmentRequestedProcessor interface {
	Handle(context.Context, events.Envelope) (application.ShipmentRequestedResult, error)
}

type Attempt struct {
	Queue               string
	Event               events.Envelope
	RetryCount          int
	Outcome, ErrorClass string
}

type AttemptObserver interface {
	Observe(context.Context, Attempt)
}
type noopAttemptObserver struct{}

func (noopAttemptObserver) Observe(context.Context, Attempt) {}

func processShipmentRequested(ctx context.Context, config RetryTopologyConfig, message deliveryMessage, delivery deliveryActions, processor ShipmentRequestedProcessor, retryPublisher RetryPublisher, observer AttemptObserver) error {
	if observer == nil {
		observer = noopAttemptObserver{}
	}
	retryCount, err := parseRetryCount(message.Headers)
	if err != nil {
		return rejectMalformed(ctx, config.Queue, message, 0, "retry_metadata", delivery, observer)
	}
	event, errorClass, classificationErr := classifyShipmentRequested(message.Body)
	if classificationErr != nil {
		if errorClass == "malformed" {
			return rejectMalformed(ctx, config.Queue, message, retryCount, errorClass, delivery, observer)
		}
		return publishAndAckDeadLetter(ctx, config.Queue, message.Body, event, retryCount, errorClass, delivery, retryPublisher, observer)
	}
	result, err := processor.Handle(ctx, event)
	if err == nil {
		if ackErr := delivery.Ack(false); ackErr != nil {
			observe(observer, ctx, config.Queue, event, retryCount, "ack_failed", "acknowledgement")
			return fmt.Errorf("acknowledge ShipmentRequested: %w", ackErr)
		}
		outcome := "processed"
		if result.Duplicate {
			outcome = "duplicate"
		}
		observe(observer, ctx, config.Queue, event, retryCount, outcome, "")
		return nil
	}
	errorClass = classifyProcessingFailure(err)
	if errorClass == "validation" {
		return publishAndAckDeadLetter(ctx, config.Queue, message.Body, event, retryCount, errorClass, delivery, retryPublisher, observer)
	}
	if retryCount >= RetryQueueCount {
		return publishAndAckDeadLetter(ctx, config.Queue, message.Body, event, retryCount, "processing", delivery, retryPublisher, observer)
	}
	if retryCount >= len(config.RetryQueues) {
		return publishAndAckDeadLetter(ctx, config.Queue, message.Body, event, retryCount, "retry_topology", delivery, retryPublisher, observer)
	}
	nextRetryCount := retryCount + 1
	retryMessage := RetryMessage{Body: append([]byte(nil), message.Body...), Event: event, Headers: retryHeaders(nextRetryCount, "processing"), RetryCount: nextRetryCount, Delay: config.RetryQueues[retryCount].Delay}
	if err := retryPublisher.PublishRetry(ctx, retryMessage); err != nil {
		observe(observer, ctx, config.Queue, event, retryCount, "retry_publication_failed", "retry_publication")
		return fmt.Errorf("publish ShipmentRequested retry: %w", err)
	}
	if err := delivery.Ack(false); err != nil {
		observe(observer, ctx, config.Queue, event, retryCount, "ack_failed", "acknowledgement")
		return fmt.Errorf("acknowledge ShipmentRequested after retry publication: %w", err)
	}
	observe(observer, ctx, config.Queue, event, retryCount, "retry_scheduled", "processing")
	return nil
}

func rejectMalformed(ctx context.Context, queue string, message deliveryMessage, retryCount int, errorClass string, delivery deliveryActions, observer AttemptObserver) error {
	event := events.Envelope{EventID: message.MessageID, EventType: message.EventType, CorrelationID: message.CorrelationID}
	if err := delivery.Reject(false); err != nil {
		observe(observer, ctx, queue, event, retryCount, "dead_letter_failed", "dead_letter")
		return fmt.Errorf("reject ShipmentRequested: %w", err)
	}
	observe(observer, ctx, queue, event, retryCount, "dead_lettered", errorClass)
	return nil
}

func publishAndAckDeadLetter(ctx context.Context, queue string, body []byte, event events.Envelope, retryCount int, errorClass string, delivery deliveryActions, publisher RetryPublisher, observer AttemptObserver) error {
	if err := publishFinalDeadLetter(ctx, publisher, body, event, retryCount, errorClass); err != nil {
		observe(observer, ctx, queue, event, retryCount, "dead_letter_publication_failed", "dead_letter")
		return fmt.Errorf("publish ShipmentRequested to DLQ: %w", err)
	}
	if err := delivery.Ack(false); err != nil {
		observe(observer, ctx, queue, event, retryCount, "ack_failed", "acknowledgement")
		return fmt.Errorf("acknowledge ShipmentRequested after DLQ publication: %w", err)
	}
	observe(observer, ctx, queue, event, retryCount, "dead_lettered", errorClass)
	return nil
}

func observe(observer AttemptObserver, ctx context.Context, queue string, event events.Envelope, retryCount int, outcome, errorClass string) {
	observer.Observe(ctx, Attempt{Queue: queue, Event: event, RetryCount: retryCount, Outcome: outcome, ErrorClass: errorClass})
}
