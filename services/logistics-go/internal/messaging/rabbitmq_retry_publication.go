package messaging

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type RetryMessage struct {
	Body       []byte
	Event      events.Envelope
	Headers    amqp.Table
	RetryCount int
	Delay      time.Duration
}

type RetryPublisher interface {
	PublishRetry(context.Context, RetryMessage) error
	PublishDeadLetter(context.Context, RetryMessage) error
}

func (p *Publisher) PublishRetry(ctx context.Context, message RetryMessage) error {
	if message.RetryCount < 1 || message.RetryCount > RetryQueueCount {
		return fmt.Errorf("retry count %d is outside the supported range", message.RetryCount)
	}
	if message.Delay <= 0 || message.Delay%time.Millisecond != 0 {
		return errors.New("retry delay must be a positive whole number of milliseconds")
	}
	return p.publish(ctx, events.ShipmentRequestedRetryRK(message.RetryCount), amqp.Publishing{
		Headers:       message.Headers,
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		Expiration:    strconv.FormatInt(message.Delay.Milliseconds(), 10),
		MessageId:     message.Event.EventID,
		Type:          message.Event.EventType,
		CorrelationId: message.Event.CorrelationID,
		Body:          message.Body,
	}, events.Exchange)
}

func (p *Publisher) PublishDeadLetter(ctx context.Context, message RetryMessage) error {
	return p.publish(ctx, events.ShipmentRequestedRK, amqp.Publishing{
		Headers:       message.Headers,
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		MessageId:     message.Event.EventID,
		Type:          message.Event.EventType,
		CorrelationId: message.Event.CorrelationID,
		Body:          message.Body,
	}, events.DeadLetterExchange)
}
