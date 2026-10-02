package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNewShipment(t *testing.T) {
	now := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.FixedZone("test", -3*60*60))

	tests := []struct {
		name    string
		orderID string
		wantErr bool
	}{
		{name: "empty order id", orderID: "", wantErr: true},
		{name: "whitespace order id", orderID: "   ", wantErr: true},
		{name: "order id too long", orderID: strings.Repeat("o", 256), wantErr: true},
		{name: "valid order id", orderID: " order-123 ", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shipment, err := NewShipment(tt.orderID, now)
			if tt.wantErr {
				if err != ErrInvalidOrderID {
					t.Fatalf("NewShipment() error = %v, want %v", err, ErrInvalidOrderID)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewShipment() unexpected error: %v", err)
			}
			if !IsValidID(shipment.ShipmentID) {
				t.Fatalf("shipment id %q is not a UUID", shipment.ShipmentID)
			}
			if shipment.OrderID != "order-123" || shipment.Status != StatusCreated {
				t.Fatalf("unexpected shipment: %+v", shipment)
			}
			if !shipment.CreatedAt.Equal(now.UTC()) || !shipment.UpdatedAt.Equal(now.UTC()) {
				t.Fatalf("timestamps = %v, %v; want %v", shipment.CreatedAt, shipment.UpdatedAt, now.UTC())
			}
		})
	}
}
