package application

import (
	"context"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type Transaction interface {
	MarkEventProcessed(ctx context.Context, event events.Envelope, processedAt time.Time) (bool, error)
	FindShipmentByOrderID(ctx context.Context, orderID string) (domain.Shipment, error)
	CreateShipment(ctx context.Context, shipment domain.Shipment) error
	AddOutboxEvent(ctx context.Context, event events.Envelope) error
}

type UnitOfWork interface {
	WithTransaction(ctx context.Context, fn func(context.Context, Transaction) error) error
}

type ShipmentRequestedHandler struct {
	unitOfWork UnitOfWork
	now        func() time.Time
	newID      func() (string, error)
}

type ShipmentRequestedResult struct {
	Duplicate bool
	Created   bool
	Shipment  domain.Shipment
	Event     *events.Envelope
}

func NewShipmentRequestedHandler(unitOfWork UnitOfWork) *ShipmentRequestedHandler {
	return &ShipmentRequestedHandler{
		unitOfWork: unitOfWork,
		now:        time.Now,
		newID:      events.NewUUID,
	}
}

func (h *ShipmentRequestedHandler) Handle(ctx context.Context, event events.Envelope) (ShipmentRequestedResult, error) {
	orderID, err := event.ShipmentRequestedOrderID()
	if err != nil {
		return ShipmentRequestedResult{}, err
	}

	result := ShipmentRequestedResult{}
	err = h.unitOfWork.WithTransaction(ctx, func(txCtx context.Context, tx Transaction) error {
		inserted, err := tx.MarkEventProcessed(txCtx, event, h.now().UTC())
		if err != nil {
			return err
		}
		if !inserted {
			result.Duplicate = true
			return nil
		}

		shipment, err := tx.FindShipmentByOrderID(txCtx, orderID)
		if err == nil {
			result.Shipment = shipment
			return nil
		}
		if err != domain.ErrNotFound {
			return err
		}

		shipmentID, err := h.newID()
		if err != nil {
			return err
		}
		shipment, err = domain.NewShipmentWithID(orderID, shipmentID, h.now())
		if err != nil {
			return err
		}
		if err := tx.CreateShipment(txCtx, shipment); err != nil {
			return err
		}

		eventID, err := h.newID()
		if err != nil {
			return err
		}
		createdEvent := events.NewEnvelope(
			eventID,
			events.ShipmentCreated,
			h.now(),
			event.CorrelationID,
			&event.EventID,
			map[string]any{
				"order_id":        shipment.OrderID,
				"shipment_id":     shipment.ShipmentID,
				"shipment_status": shipment.Status,
			},
		)
		if err := tx.AddOutboxEvent(txCtx, createdEvent); err != nil {
			return err
		}

		result.Created = true
		result.Shipment = shipment
		result.Event = &createdEvent
		return nil
	})
	if err != nil {
		return ShipmentRequestedResult{}, err
	}
	return result, nil
}
