package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type TopologyConfig struct {
	Queue           string
	DeadLetterQueue string
	RetryQueues     []RetryQueueConfig
}

type RetryMessage struct {
	Body       []byte
	Event      events.Envelope
	Headers    amqp.Table
	RetryCount int
	Delay      time.Duration
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
	mu            sync.Mutex
}

func NewPublisher(channel *amqp.Channel, timeout time.Duration) (*Publisher, error) {
	if err := channel.Confirm(false); err != nil {
		return nil, err
	}
	return &Publisher{channel: channel, confirmations: channel.NotifyPublish(make(chan amqp.Confirmation, 1)), timeout: timeout}, nil
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
	return p.publish(ctx, routingKey, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: event.EventID, Type: event.EventType, CorrelationId: event.CorrelationID, Body: body}, events.Exchange)
}

func (p *Publisher) publish(ctx context.Context, routingKey string, message amqp.Publishing, exchange string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.channel.PublishWithContext(ctx, exchange, routingKey, false, false, message); err != nil {
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
