package messaging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestStructuredAttemptLogsAreSafeAndStructured(t *testing.T) {
	secret := "postgres://user:password@example.test/db token=secret-token"
	var logs bytes.Buffer
	NewSlogAttemptObserver(slog.New(slog.NewJSONHandler(&logs, nil))).Observe(context.Background(), Attempt{Queue: "logistics.shipment.requested", Event: testEvent(), RetryCount: 1, Outcome: "retry_scheduled", ErrorClass: "processing"})
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "secret-token") {
		t.Fatalf("sensitive text leaked: %s", logs.String())
	}
	for _, field := range []string{"service", "component", "queue", "event_id", "event_type", "correlation_id", "retry_count", "outcome", "error_class"} {
		if !strings.Contains(logs.String(), `"`+field+`"`) {
			t.Fatalf("missing %q: %s", field, logs.String())
		}
	}
	var unsafe bytes.Buffer
	NewSlogAttemptObserver(slog.New(slog.NewJSONHandler(&unsafe, nil))).Observe(context.Background(), Attempt{Queue: "queue with secret", Event: events.Envelope{EventID: "not-a-uuid", EventType: "unexpected"}})
	for _, value := range []string{"queue with secret", "not-a-uuid", "unexpected"} {
		if strings.Contains(unsafe.String(), value) {
			t.Fatalf("unsafe value %q appeared: %s", value, unsafe.String())
		}
	}
}

func testEvent() events.Envelope {
	return events.Envelope{EventID: "11111111-1111-1111-1111-111111111111", EventType: events.ShipmentRequested, CorrelationID: "22222222-2222-2222-2222-222222222222"}
}
