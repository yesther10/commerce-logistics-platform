package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestClassifyShipmentRequestedFailures(t *testing.T) {
	cases := []struct {
		name, body, wantClass string
		wantEvent, wantError  bool
	}{
		{"malformed", `{"event_id":`, "malformed", false, true},
		{"invalid payload", string(classificationEnvelope(t, map[string]any{})), "validation", true, true},
		{"unsupported version", `{"event_id":"11111111-1111-1111-1111-111111111111","event_type":"ShipmentRequested","event_version":2,"occurred_at":"2026-01-01T00:00:00Z","correlation_id":"22222222-2222-2222-2222-222222222222","causation_id":null,"payload":{"order_id":"order-1"}}`, "validation", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event, class, err := classifyShipmentRequested([]byte(tc.body))
			if class != tc.wantClass || (err != nil) != tc.wantError {
				t.Fatalf("classification = %q, error = %v", class, err)
			}
			if tc.wantEvent && event.EventID == "" {
				t.Fatal("classification lost event identity")
			}
		})
	}
}

func TestRetryMetadataIsBoundedAndFinalDLQPublicationCopiesBody(t *testing.T) {
	body := classificationEnvelope(t, map[string]any{"order_id": "order-1"})
	event, _, err := classifyShipmentRequested(body)
	if err != nil {
		t.Fatal(err)
	}
	publisher := &classificationPublisher{}
	if err := publishFinalDeadLetter(context.Background(), publisher, body, event, 2, "validation"); err != nil {
		t.Fatal(err)
	}
	if len(publisher.messages) != 1 || string(publisher.messages[0].Body) != string(body) {
		t.Fatalf("publication = %+v", publisher.messages)
	}
	if len(publisher.messages[0].Headers) != 3 || publisher.messages[0].Headers[retryCountHeader] != int32(2) {
		t.Fatalf("headers = %+v", publisher.messages[0].Headers)
	}
	if publisher.messages[0].Event.EventID != event.EventID || publisher.messages[0].Event.CorrelationID != event.CorrelationID {
		t.Fatalf("publication changed identity: %+v", publisher.messages[0].Event)
	}
	body[0] = 'x'
	if publisher.messages[0].Body[0] == 'x' {
		t.Fatal("published body aliases input")
	}
}

func TestValidationClassificationPreservesOriginalBodyAndUsesSafeClass(t *testing.T) {
	body := classificationEnvelope(t, map[string]any{})
	event, class, err := classifyShipmentRequested(body)
	if class != "validation" || err == nil {
		t.Fatalf("classification = %q, error = %v", class, err)
	}
	publisher := &classificationPublisher{}
	if err := publishFinalDeadLetter(context.Background(), publisher, body, event, 0, class); err != nil {
		t.Fatal(err)
	}
	message := publisher.messages[0]
	if string(message.Body) != string(body) || message.RetryCount != 0 || message.Headers[retryLastErrorHeader] != "validation" || message.Headers[retryErrorClassHeader] != "validation" {
		t.Fatalf("validation DLQ message = %+v", message)
	}
}

func TestParseRetryCountRejectsUnsafeValues(t *testing.T) {
	for _, value := range []any{-1, RetryQueueCount + 1, "not-a-number", true} {
		if _, err := parseRetryCount(amqp.Table{retryCountHeader: value}); err == nil {
			t.Fatalf("parseRetryCount(%#v) accepted invalid value", value)
		}
	}
}

func TestParseRetryCountAcceptsBrokerHeaderTypes(t *testing.T) {
	for _, value := range []any{0, int32(1), int64(2), "3"} {
		if count, err := parseRetryCount(amqp.Table{retryCountHeader: value}); err != nil || count < 0 || count > RetryQueueCount {
			t.Fatalf("parseRetryCount(%#v) = %d, %v", value, count, err)
		}
	}
}

func TestClassificationUsesSafeErrorClasses(t *testing.T) {
	if safeErrorClass("postgres://secret") != "unknown" || safeErrorClass("validation") != "validation" {
		t.Fatal("unsafe error class was not bounded")
	}
	if classifyProcessingFailure(domain.ErrInvalidOrderID) != "validation" || classifyProcessingFailure(errors.New("temporary")) != "processing" {
		t.Fatal("processing failure classification changed")
	}
}

func TestRetryHeadersContainOnlyBoundedMetadata(t *testing.T) {
	headers := retryHeaders(3, "postgres://user:password@example.test/db")
	if len(headers) != 3 || headers[retryCountHeader] != int32(3) || headers[retryLastErrorHeader] != "unknown" || headers[retryErrorClassHeader] != "unknown" {
		t.Fatalf("bounded headers = %+v", headers)
	}
	for key := range headers {
		if key == "authorization" || key == "payload" {
			t.Fatalf("unsafe header copied: %q", key)
		}
	}
}

func TestFinalDeadLetterPublicationPropagatesPublisherFailure(t *testing.T) {
	wantErr := errors.New("publisher confirmation timed out")
	publisher := &classificationPublisher{err: wantErr}
	err := publishFinalDeadLetter(context.Background(), publisher, classificationEnvelope(t, map[string]any{"order_id": "order-1"}), events.Envelope{}, 0, "validation")
	if !errors.Is(err, wantErr) {
		t.Fatalf("publishFinalDeadLetter() error = %v, want %v", err, wantErr)
	}
}

func classificationEnvelope(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	body, err := events.NewEnvelope("11111111-1111-1111-1111-111111111111", events.ShipmentRequested, time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), "22222222-2222-2222-2222-222222222222", nil, payload).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return body
}

type classificationPublisher struct {
	messages []RetryMessage
	err      error
}

func (p *classificationPublisher) PublishRetry(context.Context, RetryMessage) error {
	return errors.New("unexpected retry publication")
}

func (p *classificationPublisher) PublishDeadLetter(_ context.Context, message RetryMessage) error {
	p.messages = append(p.messages, message)
	return p.err
}
