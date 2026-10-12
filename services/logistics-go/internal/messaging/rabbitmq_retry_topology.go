package messaging

import (
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

const RetryQueueCount = 3

type RetryQueueConfig struct {
	Queue string
	Delay time.Duration
}

// RetryTopologyConfig is additive to the original queue and DLQ topology.
type RetryTopologyConfig struct {
	Queue           string
	DeadLetterQueue string
	RetryQueues     []RetryQueueConfig
}

// DeclareRetryTopology declares only the new retry queues. The established
// consumer queue and final DLQ remain owned by DeclareLogisticsTopology.
func DeclareRetryTopology(channel *amqp.Channel, config RetryTopologyConfig) error {
	if err := validateRetryQueues(config.RetryQueues); err != nil {
		return err
	}
	for retryNumber, retryQueue := range config.RetryQueues {
		if _, err := channel.QueueDeclare(retryQueue.Queue, true, false, false, false, amqp.Table{
			"x-dead-letter-exchange":    events.Exchange,
			"x-dead-letter-routing-key": events.ShipmentRequestedRK,
		}); err != nil {
			return err
		}
		if err := channel.QueueBind(retryQueue.Queue, events.ShipmentRequestedRetryRK(retryNumber+1), events.Exchange, false, nil); err != nil {
			return err
		}
	}
	return nil
}

func validateRetryQueues(retryQueues []RetryQueueConfig) error {
	if len(retryQueues) != RetryQueueCount {
		return fmt.Errorf("expected %d ShipmentRequested retry queues, got %d", RetryQueueCount, len(retryQueues))
	}
	previousMilliseconds := int64(0)
	for index, retryQueue := range retryQueues {
		if retryQueue.Queue == "" {
			return fmt.Errorf("retry queue %d must have a name", index+1)
		}
		if retryQueue.Delay <= 0 || retryQueue.Delay%time.Millisecond != 0 {
			return fmt.Errorf("retry queue %q must have a positive whole-millisecond delay", retryQueue.Queue)
		}
		milliseconds := retryQueue.Delay.Milliseconds()
		if index > 0 && milliseconds <= previousMilliseconds {
			return errors.New("ShipmentRequested retry delays must be strictly increasing at broker millisecond granularity")
		}
		previousMilliseconds = milliseconds
	}
	return nil
}
