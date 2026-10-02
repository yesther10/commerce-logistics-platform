package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestPublishPendingMarksOnlyConfirmedEvents(t *testing.T) {
	event := events.NewEnvelope(
		"11111111-1111-1111-1111-111111111111",
		events.ShipmentCreated,
		time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC),
		"22222222-2222-2222-2222-222222222222",
		nil,
		map[string]any{"order_id": "order-1"},
	)
	store := &fakeOutboxStore{events: []events.Envelope{event}}
	publisher := &fakeEventPublisher{}

	count, err := PublishPending(context.Background(), store, publisher, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(store.published) != 1 || len(publisher.published) != 1 {
		t.Fatalf("count = %d, marked = %d, published = %d", count, len(store.published), len(publisher.published))
	}
}

func TestPublishPendingLeavesTheEventPendingWhenConfirmationFails(t *testing.T) {
	event := events.NewEnvelope(
		"11111111-1111-1111-1111-111111111111",
		events.ShipmentCreated,
		time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC),
		"22222222-2222-2222-2222-222222222222",
		nil,
		map[string]any{"order_id": "order-1"},
	)
	store := &fakeOutboxStore{events: []events.Envelope{event}}
	publisher := &fakeEventPublisher{err: errors.New("publisher confirmation timed out")}

	count, err := PublishPending(context.Background(), store, publisher, 10)
	if !errors.Is(err, publisher.err) || count != 0 || len(store.published) != 0 {
		t.Fatalf("count = %d, err = %v, marked = %d", count, err, len(store.published))
	}
}

type fakeOutboxStore struct {
	events    []events.Envelope
	published []string
}

func (f *fakeOutboxStore) PendingOutbox(_ context.Context, _ int) ([]events.Envelope, error) {
	return f.events, nil
}

func (f *fakeOutboxStore) MarkOutboxPublished(_ context.Context, eventID string) error {
	f.published = append(f.published, eventID)
	return nil
}

type fakeEventPublisher struct {
	published []events.Envelope
	err       error
}

func (f *fakeEventPublisher) Publish(_ context.Context, event events.Envelope) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, event)
	return nil
}
