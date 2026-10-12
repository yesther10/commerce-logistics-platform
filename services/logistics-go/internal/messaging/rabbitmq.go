package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/application"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type TopologyConfig struct {
	Queue           string
	DeadLetterQueue string
	RetryQueues     []RetryQueueConfig
}

func DeclareLogisticsTopology(channel *amqp.Channel, config TopologyConfig) error {
	if err := channel.ExchangeDeclare(events.Exchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return err
	}
	if err := channel.ExchangeDeclare(events.DeadLetterExchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := channel.QueueDeclare(config.Queue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    events.DeadLetterExchange,
		"x-dead-letter-routing-key": events.ShipmentRequestedRK,
	}); err != nil {
		return err
	}
	if err := channel.QueueBind(config.Queue, events.ShipmentRequestedRK, events.Exchange, false, nil); err != nil {
		return err
	}
	if _, err := channel.QueueDeclare(config.DeadLetterQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := channel.QueueBind(config.DeadLetterQueue, events.ShipmentRequestedRK, events.DeadLetterExchange, false, nil); err != nil {
		return err
	}
	if len(config.RetryQueues) == 0 {
		return nil
	}
	return DeclareRetryTopology(channel, RetryTopologyConfig{
		Queue:           config.Queue,
		DeadLetterQueue: config.DeadLetterQueue,
		RetryQueues:     config.RetryQueues,
	})
}

type Publisher struct {
	channel       *amqp.Channel
	confirmations <-chan amqp.Confirmation
	timeout       time.Duration
}

func NewPublisher(channel *amqp.Channel, timeout time.Duration) (*Publisher, error) {
	if err := channel.Confirm(false); err != nil {
		return nil, err
	}
	confirmations := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	return &Publisher{channel: channel, confirmations: confirmations, timeout: timeout}, nil
}

func (p *Publisher) Publish(ctx context.Context, event events.Envelope) error {
	routingKey, err := event.RoutingKey()
	if err != nil {
		return err
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := p.channel.PublishWithContext(ctx, events.Exchange, routingKey, false, false, amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		MessageId:     event.EventID,
		Type:          event.EventType,
		CorrelationId: event.CorrelationID,
		Body:          body,
	}); err != nil {
		return err
	}

	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case confirmation, ok := <-p.confirmations:
		if !ok {
			return errors.New("publisher confirmation channel closed")
		}
		if !confirmation.Ack {
			return errors.New("RabbitMQ negatively acknowledged event publication")
		}
		return nil
	case <-timer.C:
		return errors.New("RabbitMQ publisher confirmation timed out")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ConsumeShipmentRequested(ctx context.Context, channel *amqp.Channel, config TopologyConfig, processor interface {
	Handle(context.Context, events.Envelope) (application.ShipmentRequestedResult, error)
}) error {
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
			event, err := events.Parse(delivery.Body)
			if err == nil {
				_, err = processor.Handle(ctx, event)
			}
			if err != nil {
				_ = delivery.Reject(false)
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge ShipmentRequested: %w", err)
			}
		}
	}
}
