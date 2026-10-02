package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	Exchange            = "platform.events"
	DeadLetterExchange  = "platform.events.dlx"
	ShipmentRequested   = "ShipmentRequested"
	ShipmentCreated     = "ShipmentCreated"
	ShipmentRequestedRK = "commerce.shipment.requested"
	ShipmentCreatedRK   = "logistics.shipment.created"
)

type Envelope struct {
	EventID       string
	EventType     string
	EventVersion  int
	OccurredAt    time.Time
	CorrelationID string
	CausationID   *string
	Payload       map[string]any
}

func NewEnvelope(eventID, eventType string, occurredAt time.Time, correlationID string, causationID *string, payload map[string]any) Envelope {
	return Envelope{
		EventID:       eventID,
		EventType:     eventType,
		EventVersion:  1,
		OccurredAt:    occurredAt.UTC(),
		CorrelationID: correlationID,
		CausationID:   causationID,
		Payload:       payload,
	}
}

func NewUUID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(id[0:4]),
		hex.EncodeToString(id[4:6]),
		hex.EncodeToString(id[6:8]),
		hex.EncodeToString(id[8:10]),
		hex.EncodeToString(id[10:16]),
	), nil
}

func (e Envelope) RoutingKey() (string, error) {
	switch e.EventType {
	case ShipmentCreated:
		return ShipmentCreatedRK, nil
	case ShipmentRequested:
		return ShipmentRequestedRK, nil
	default:
		return "", fmt.Errorf("unsupported event type %q", e.EventType)
	}
}

func (e Envelope) Validate(expectedType string) error {
	if e.EventType != expectedType {
		return fmt.Errorf("expected event type %q, got %q", expectedType, e.EventType)
	}
	if e.EventVersion != 1 {
		return fmt.Errorf("unsupported event version %d", e.EventVersion)
	}
	if !isUUID(e.EventID) || !isUUID(e.CorrelationID) {
		return errors.New("event_id and correlation_id must be UUIDs")
	}
	if e.CausationID != nil && !isUUID(*e.CausationID) {
		return errors.New("causation_id must be a UUID or null")
	}
	if e.OccurredAt.IsZero() || e.OccurredAt.Location() == time.Local {
		return errors.New("occurred_at must be an ISO 8601 timestamp")
	}
	if e.Payload == nil {
		return errors.New("payload must be an object")
	}
	return nil
}

func (e Envelope) ShipmentRequestedOrderID() (string, error) {
	if err := e.Validate(ShipmentRequested); err != nil {
		return "", err
	}
	orderID, ok := e.Payload["order_id"].(string)
	if !ok || strings.TrimSpace(orderID) == "" {
		return "", errors.New("ShipmentRequested payload must include a non-empty string order_id")
	}
	return strings.TrimSpace(orderID), nil
}

func (e Envelope) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		EventID       string         `json:"event_id"`
		EventType     string         `json:"event_type"`
		EventVersion  int            `json:"event_version"`
		OccurredAt    string         `json:"occurred_at"`
		CorrelationID string         `json:"correlation_id"`
		CausationID   *string        `json:"causation_id"`
		Payload       map[string]any `json:"payload"`
	}{
		EventID:       e.EventID,
		EventType:     e.EventType,
		EventVersion:  e.EventVersion,
		OccurredAt:    e.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
		CorrelationID: e.CorrelationID,
		CausationID:   e.CausationID,
		Payload:       e.Payload,
	})
}

func (e *Envelope) UnmarshalJSON(data []byte) error {
	var raw struct {
		EventID       string          `json:"event_id"`
		EventType     string          `json:"event_type"`
		EventVersion  int             `json:"event_version"`
		OccurredAt    string          `json:"occurred_at"`
		CorrelationID string          `json:"correlation_id"`
		CausationID   *string         `json:"causation_id"`
		Payload       json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode event envelope: %w", err)
	}
	occurredAt, err := time.Parse(time.RFC3339, raw.OccurredAt)
	if err != nil {
		return fmt.Errorf("decode occurred_at: %w", err)
	}
	var payload map[string]any
	if len(raw.Payload) == 0 || string(raw.Payload) == "null" || json.Unmarshal(raw.Payload, &payload) != nil || payload == nil {
		return errors.New("payload must be a JSON object")
	}
	*e = Envelope{
		EventID:       raw.EventID,
		EventType:     raw.EventType,
		EventVersion:  raw.EventVersion,
		OccurredAt:    occurredAt.UTC(),
		CorrelationID: raw.CorrelationID,
		CausationID:   raw.CausationID,
		Payload:       payload,
	}
	return nil
}

func Parse(data []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func isUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16
}
