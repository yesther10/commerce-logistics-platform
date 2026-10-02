package application

import (
	"context"
	"testing"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestShipmentRequestedIsTransactionalAndIdempotent(t *testing.T) {
	store := newFakeUnitOfWork()
	handler := NewShipmentRequestedHandler(store)
	handler.now = func() time.Time { return time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC) }
	ids := []string{
		"33333333-3333-3333-3333-333333333333",
		"44444444-4444-4444-4444-444444444444",
	}
	handler.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	event := events.NewEnvelope(
		"11111111-1111-1111-1111-111111111111", events.ShipmentRequested, handler.now(),
		"22222222-2222-2222-2222-222222222222", nil, map[string]any{"order_id": "order-1"},
	)

	first, err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.Event == nil || len(store.outbox) != 1 || len(store.processed) != 1 {
		t.Fatalf("first result = %+v, processed = %d, outbox = %d", first, len(store.processed), len(store.outbox))
	}
	if first.Event.CausationID == nil || *first.Event.CausationID != event.EventID {
		t.Fatalf("causation_id = %v", first.Event.CausationID)
	}

	second, err := handler.Handle(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate || len(store.outbox) != 1 || len(store.shipments) != 1 {
		t.Fatalf("duplicate result = %+v, shipments = %d, outbox = %d", second, len(store.shipments), len(store.outbox))
	}
}

type fakeUnitOfWork struct {
	processed map[string]bool
	shipments map[string]domain.Shipment
	outbox    []events.Envelope
}

func newFakeUnitOfWork() *fakeUnitOfWork {
	return &fakeUnitOfWork{processed: map[string]bool{}, shipments: map[string]domain.Shipment{}}
}

func (f *fakeUnitOfWork) WithTransaction(ctx context.Context, fn func(context.Context, Transaction) error) error {
	return fn(ctx, f)
}

func (f *fakeUnitOfWork) MarkEventProcessed(_ context.Context, event events.Envelope, _ time.Time) (bool, error) {
	if f.processed[event.EventID] {
		return false, nil
	}
	f.processed[event.EventID] = true
	return true, nil
}

func (f *fakeUnitOfWork) FindShipmentByOrderID(_ context.Context, orderID string) (domain.Shipment, error) {
	for _, shipment := range f.shipments {
		if shipment.OrderID == orderID {
			return shipment, nil
		}
	}
	return domain.Shipment{}, domain.ErrNotFound
}

func (f *fakeUnitOfWork) CreateShipment(_ context.Context, shipment domain.Shipment) error {
	f.shipments[shipment.ShipmentID] = shipment
	return nil
}

func (f *fakeUnitOfWork) AddOutboxEvent(_ context.Context, event events.Envelope) error {
	f.outbox = append(f.outbox, event)
	return nil
}
