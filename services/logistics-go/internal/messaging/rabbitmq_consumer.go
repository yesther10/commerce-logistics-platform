package messaging

import (
	"context"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/application"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type consumerOptions struct {
	retryPublisher RetryPublisher
	observer       AttemptObserver
}
type ConsumerOption func(*consumerOptions)

func WithRetryPublisher(publisher RetryPublisher) ConsumerOption {
	return func(options *consumerOptions) { options.retryPublisher = publisher }
}

func ConsumeShipmentRequested(ctx context.Context, channel *amqp.Channel, config TopologyConfig, processor interface {
	Handle(context.Context, events.Envelope) (application.ShipmentRequestedResult, error)
}, options ...ConsumerOption) error {
	consumerOptions := consumerOptions{observer: noopAttemptObserver{}}
	for _, option := range options {
		option(&consumerOptions)
	}
	if err := DeclareLogisticsTopology(channel, config); err != nil {
		return fmt.Errorf("declare logistics RabbitMQ topology: %w", err)
	}
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set RabbitMQ prefetch: %w", err)
	}
	deliveries, err := channel.Consume(config.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume ShipmentRequested: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("RabbitMQ consumer channel closed")
			}
			if consumerOptions.retryPublisher == nil {
				event, parseErr := events.Parse(delivery.Body)
				if parseErr == nil {
					_, parseErr = processor.Handle(ctx, event)
				}
				if parseErr != nil {
					_ = delivery.Reject(false)
					continue
				}
				if ackErr := delivery.Ack(false); ackErr != nil {
					return fmt.Errorf("acknowledge ShipmentRequested: %w", ackErr)
				}
				continue
			}
			message := deliveryMessage{Body: delivery.Body, Headers: delivery.Headers, MessageID: delivery.MessageId, EventType: delivery.Type, CorrelationID: delivery.CorrelationId}
			retryConfig := RetryTopologyConfig{Queue: config.Queue, DeadLetterQueue: config.DeadLetterQueue, RetryQueues: config.RetryQueues}
			if err := processShipmentRequested(ctx, retryConfig, message, deliveryAdapter{delivery}, processor, consumerOptions.retryPublisher, consumerOptions.observer); err != nil {
				return err
			}
		}
	}
}

type deliveryAdapter struct{ delivery amqp.Delivery }

func (d deliveryAdapter) Ack(multiple bool) error   { return d.delivery.Ack(multiple) }
func (d deliveryAdapter) Reject(requeue bool) error { return d.delivery.Reject(requeue) }
