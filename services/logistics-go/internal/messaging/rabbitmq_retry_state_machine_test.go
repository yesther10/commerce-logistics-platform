package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/application"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestShipmentRequestedRetryProgressionAndExhaustion(t *testing.T) {
	for count := 0; count <= RetryQueueCount; count++ {
		delivery, publisher := &fakeDelivery{}, &fakeRetryPublisher{}
		err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: envelopeBody(t, map[string]any{"order_id": "order-1"}), Headers: retryHeaders(count, "processing")}, delivery, fakeProcessor{err: errors.New("temporary failure")}, publisher, nil)
		if err != nil {
			t.Fatal(err)
		}
		if delivery.acked != 1 || delivery.rejected != 0 {
			t.Fatalf("delivery = %+v", delivery)
		}
		if count == RetryQueueCount {
			if len(publisher.deadLetters) != 1 {
				t.Fatalf("DLQ publications = %d", len(publisher.deadLetters))
			}
			continue
		}
		if len(publisher.messages) != 1 || publisher.messages[0].RetryCount != count+1 || publisher.messages[0].Delay != testTopology().RetryQueues[count].Delay {
			t.Fatalf("retry = %+v", publisher.messages)
		}
	}
}

func TestShipmentRequestedValidationDuplicateAndFailureOrdering(t *testing.T) {
	t.Run("duplicate is acknowledged", func(t *testing.T) {
		delivery, publisher := &fakeDelivery{}, &fakeRetryPublisher{}
		if err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: envelopeBody(t, map[string]any{"order_id": "order-1"})}, delivery, fakeProcessor{result: application.ShipmentRequestedResult{Duplicate: true}}, publisher, nil); err != nil {
			t.Fatal(err)
		}
		if delivery.acked != 1 || len(publisher.messages) != 0 {
			t.Fatalf("delivery = %+v, publications = %+v", delivery, publisher)
		}
	})
	t.Run("publication failure leaves delivery unacknowledged", func(t *testing.T) {
		delivery, publisher := &fakeDelivery{}, &fakeRetryPublisher{err: errors.New("confirmation timed out")}
		err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: envelopeBody(t, map[string]any{"order_id": "order-1"})}, delivery, fakeProcessor{err: errors.New("temporary failure")}, publisher, nil)
		if !errors.Is(err, publisher.err) || delivery.acked != 0 || delivery.rejected != 0 {
			t.Fatalf("err = %v, delivery = %+v", err, delivery)
		}
	})
	t.Run("ack failure follows publication", func(t *testing.T) {
		delivery, publisher := &fakeDelivery{ackErr: errors.New("ack failed")}, &fakeRetryPublisher{}
		err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: envelopeBody(t, map[string]any{"order_id": "order-1"})}, delivery, fakeProcessor{err: errors.New("temporary failure")}, publisher, nil)
		if !errors.Is(err, delivery.ackErr) || delivery.acked != 1 || len(publisher.messages) != 1 {
			t.Fatalf("err = %v, delivery = %+v", err, delivery)
		}
	})
}

func TestShipmentRequestedMalformedMessageUsesBrokerDLQ(t *testing.T) {
	delivery, publisher := &fakeDelivery{}, &fakeRetryPublisher{}
	err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: []byte(`{"event_id":`), Headers: map[string]any{retryCountHeader: int32(2), "authorization": "Bearer secret-token"}, MessageID: "message-id", EventType: events.ShipmentRequested, CorrelationID: "correlation-id"}, delivery, fakeProcessor{}, publisher, nil)
	if err != nil || delivery.rejected != 1 || delivery.acked != 0 || len(publisher.deadLetters) != 0 {
		t.Fatalf("err = %v, delivery = %+v, publisher = %+v", err, delivery, publisher)
	}
}

func TestShipmentRequestedDLQAckFailureHappensAfterPublication(t *testing.T) {
	delivery, publisher := &fakeDelivery{ackErr: errors.New("ack failed")}, &fakeRetryPublisher{}
	err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: envelopeBody(t, map[string]any{"order_id": "order-1"}), Headers: retryHeaders(RetryQueueCount, "processing")}, delivery, fakeProcessor{err: errors.New("temporary failure")}, publisher, nil)
	if !errors.Is(err, delivery.ackErr) || delivery.acked != 1 || len(publisher.deadLetters) != 1 {
		t.Fatalf("err = %v, delivery = %+v, publisher = %+v", err, delivery, publisher)
	}
}

func TestShipmentRequestedRetryMetadataDoesNotCopyIncomingHeaders(t *testing.T) {
	body := envelopeBody(t, map[string]any{"order_id": "order-1"})
	publisher := &fakeRetryPublisher{}
	err := processShipmentRequested(context.Background(), testTopology(), deliveryMessage{Body: body, Headers: map[string]any{"authorization": "Bearer secret-token"}}, &fakeDelivery{}, fakeProcessor{err: errors.New("postgres://secret")}, publisher, nil)
	if err != nil || len(publisher.messages) != 1 {
		t.Fatalf("err = %v, publisher = %+v", err, publisher)
	}
	for key, value := range publisher.messages[0].Headers {
		if key == "authorization" || value == "Bearer secret-token" || value == "postgres://secret" {
			t.Fatalf("sensitive metadata = %q: %#v", key, value)
		}
	}
}

func testTopology() RetryTopologyConfig {
	return RetryTopologyConfig{Queue: "logistics.shipment.requested", DeadLetterQueue: "logistics.shipment.requested.dlq", RetryQueues: []RetryQueueConfig{{"retry.1", time.Second}, {"retry.2", 5 * time.Second}, {"retry.3", 15 * time.Second}}}
}

func envelopeBody(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	body, err := events.NewEnvelope("11111111-1111-1111-1111-111111111111", events.ShipmentRequested, time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), "22222222-2222-2222-2222-222222222222", nil, payload).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return body
}

type fakeDelivery struct {
	acked, rejected   int
	ackErr, rejectErr error
}

func (f *fakeDelivery) Ack(bool) error    { f.acked++; return f.ackErr }
func (f *fakeDelivery) Reject(bool) error { f.rejected++; return f.rejectErr }

type fakeRetryPublisher struct {
	messages, deadLetters []RetryMessage
	err                   error
}

func (f *fakeRetryPublisher) PublishRetry(_ context.Context, message RetryMessage) error {
	f.messages = append(f.messages, message)
	return f.err
}
func (f *fakeRetryPublisher) PublishDeadLetter(_ context.Context, message RetryMessage) error {
	f.deadLetters = append(f.deadLetters, message)
	return f.err
}

type fakeProcessor struct {
	result application.ShipmentRequestedResult
	err    error
}

func (f fakeProcessor) Handle(context.Context, events.Envelope) (application.ShipmentRequestedResult, error) {
	return f.result, f.err
}
