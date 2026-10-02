package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEnvelopeUsesThePublishedContract(t *testing.T) {
	event := NewEnvelope(
		"11111111-1111-1111-1111-111111111111",
		ShipmentRequested,
		time.Date(2026, time.January, 1, 12, 0, 0, 0, time.FixedZone("test", -3*60*60)),
		"22222222-2222-2222-2222-222222222222",
		nil,
		map[string]any{"order_id": "order-1"},
	)

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"event_id":"11111111-1111-1111-1111-111111111111","event_type":"ShipmentRequested","event_version":1,"occurred_at":"2026-01-01T15:00:00Z","correlation_id":"22222222-2222-2222-2222-222222222222","causation_id":null,"payload":{"order_id":"order-1"}}`
	if string(data) != want {
		t.Fatalf("envelope = %s, want %s", data, want)
	}
	if key, _ := event.RoutingKey(); key != ShipmentRequestedRK {
		t.Fatalf("routing key = %q", key)
	}
}

func TestParseRejectsUnsupportedEnvelope(t *testing.T) {
	event, err := Parse([]byte(`{"event_id":"11111111-1111-1111-1111-111111111111","event_type":"ShipmentRequested","event_version":2,"occurred_at":"2026-01-01T00:00:00Z","correlation_id":"22222222-2222-2222-2222-222222222222","causation_id":null,"payload":{"order_id":"order-1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Validate(ShipmentRequested); err == nil {
		t.Fatal("Validate() accepted unsupported version")
	}
}
