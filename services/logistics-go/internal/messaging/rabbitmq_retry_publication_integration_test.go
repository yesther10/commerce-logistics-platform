package messaging

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestRabbitMQRetryTopologyIntegration(t *testing.T) {
	if os.Getenv("RABBITMQ_INTEGRATION") != "1" {
		t.Skip("set RABBITMQ_INTEGRATION=1 to run against RabbitMQ")
	}
	connection, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	channel, err := connection.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	config := RetryTopologyConfig{
		Queue: "issue30.verify." + suffix, DeadLetterQueue: "issue30.verify.dlq." + suffix,
		RetryQueues: []RetryQueueConfig{{"issue30.verify.retry.1." + suffix, 50 * time.Millisecond}, {"issue30.verify.retry.2." + suffix, 100 * time.Millisecond}, {"issue30.verify.retry.3." + suffix, 150 * time.Millisecond}},
	}
	defer deleteVerificationQueues(channel, config)
	if err := DeclareLogisticsTopology(channel, TopologyConfig{Queue: config.Queue, DeadLetterQueue: config.DeadLetterQueue, RetryQueues: config.RetryQueues}); err != nil {
		t.Fatal(err)
	}
	publisher, err := NewPublisher(channel, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	body := publicationEnvelopeBody(t)
	event, err := events.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	for retryCount, delay := range []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 150 * time.Millisecond} {
		headers := amqp.Table{"x-logistics-retry-count": int32(retryCount + 1), "x-logistics-retry-last-error": "processing", "x-logistics-retry-error-class": "processing"}
		if err := publisher.PublishRetry(context.Background(), RetryMessage{Body: body, Event: event, Headers: headers, RetryCount: retryCount + 1, Delay: delay}); err != nil {
			t.Fatal(err)
		}
		delivery := getEventually(t, channel, config.Queue, 2*time.Second)
		if delivery.RoutingKey != events.ShipmentRequestedRK || string(delivery.Body) != string(body) {
			t.Fatalf("retry %d delivery = %+v", retryCount+1, delivery)
		}
		if delivery.Headers["x-logistics-retry-count"] != int32(retryCount+1) {
			t.Fatalf("retry %d headers = %+v", retryCount+1, delivery.Headers)
		}
		if delivery.Headers["x-logistics-retry-last-error"] != "processing" || delivery.Headers["x-logistics-retry-error-class"] != "processing" {
			t.Fatalf("retry %d safe headers = %+v", retryCount+1, delivery.Headers)
		}
		if err := delivery.Ack(false); err != nil {
			t.Fatal(err)
		}
	}
	headers := amqp.Table{"x-logistics-retry-count": int32(RetryQueueCount), "x-logistics-retry-last-error": "processing", "x-logistics-retry-error-class": "processing"}
	if err := publisher.PublishDeadLetter(context.Background(), RetryMessage{Body: body, Event: event, Headers: headers, RetryCount: RetryQueueCount}); err != nil {
		t.Fatal(err)
	}
	deadLetter := getEventually(t, channel, config.DeadLetterQueue, 2*time.Second)
	if deadLetter.RoutingKey != events.ShipmentRequestedRK || string(deadLetter.Body) != string(body) || deadLetter.MessageId != event.EventID || deadLetter.Type != event.EventType || deadLetter.CorrelationId != event.CorrelationID {
		t.Fatalf("final DLQ delivery = %+v", deadLetter)
	}
	if deadLetter.Headers["x-logistics-retry-count"] != int32(RetryQueueCount) || deadLetter.Headers["x-logistics-retry-last-error"] != "processing" || deadLetter.Headers["x-logistics-retry-error-class"] != "processing" {
		t.Fatalf("final DLQ safe headers = %+v", deadLetter.Headers)
	}
	if err := deadLetter.Ack(false); err != nil {
		t.Fatal(err)
	}
}

func getEventually(t *testing.T, channel *amqp.Channel, queue string, timeout time.Duration) amqp.Delivery {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		delivery, ok, err := channel.Get(queue, false)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return delivery
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for a message in %s", queue)
	return amqp.Delivery{}
}

func deleteVerificationQueues(channel *amqp.Channel, config RetryTopologyConfig) {
	for _, queue := range append([]string{config.Queue, config.DeadLetterQueue}, retryQueueNames(config.RetryQueues)...) {
		_, _ = channel.QueueDelete(queue, false, false, false)
	}
}

func retryQueueNames(queues []RetryQueueConfig) []string {
	names := make([]string, 0, len(queues))
	for _, queue := range queues {
		names = append(names, queue.Queue)
	}
	return names
}

func publicationEnvelopeBody(t *testing.T) []byte {
	t.Helper()
	body, err := events.NewEnvelope("11111111-1111-1111-1111-111111111111", events.ShipmentRequested, time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), "22222222-2222-2222-2222-222222222222", nil, map[string]any{"order_id": "issue-30-verification"}).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return body
}
