package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
)

type fakeRepository struct {
	shipments map[string]domain.Shipment
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{shipments: make(map[string]domain.Shipment)}
}

func (r *fakeRepository) Create(_ context.Context, shipment domain.Shipment) error {
	r.shipments[shipment.ShipmentID] = shipment
	return nil
}

func (r *fakeRepository) GetByID(_ context.Context, shipmentID string) (domain.Shipment, error) {
	shipment, ok := r.shipments[shipmentID]
	if !ok {
		return domain.Shipment{}, domain.ErrNotFound
	}
	return shipment, nil
}

func TestHealthz(t *testing.T) {
	response := request(t, NewHandler(newFakeRepository()), http.MethodGet, "/healthz", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestCreateShipment(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "missing order id", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "blank order id", body: `{"order_id":"   "}`, wantStatus: http.StatusBadRequest},
		{name: "malformed json", body: `{"order_id":`, wantStatus: http.StatusBadRequest},
		{name: "valid order", body: `{"order_id":"order-123"}`, wantStatus: http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := newFakeRepository()
			response := request(t, NewHandler(repository), http.MethodPost, "/shipments", tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tt.wantStatus, response.Body.String())
			}
			if tt.wantStatus != http.StatusCreated {
				return
			}

			var shipment domain.Shipment
			if err := json.NewDecoder(response.Body).Decode(&shipment); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !domain.IsValidID(shipment.ShipmentID) || shipment.OrderID != "order-123" || shipment.Status != domain.StatusCreated {
				t.Fatalf("unexpected shipment: %+v", shipment)
			}
			if response.Header().Get("Location") != "/shipments/"+shipment.ShipmentID {
				t.Fatalf("location = %q", response.Header().Get("Location"))
			}
			if len(repository.shipments) != 1 {
				t.Fatalf("stored shipments = %d, want 1", len(repository.shipments))
			}
		})
	}
}

func TestGetShipment(t *testing.T) {
	repository := newFakeRepository()
	shipment := domain.Shipment{
		ShipmentID: "123e4567-e89b-12d3-a456-426614174000",
		OrderID:    "order-123",
		Status:     domain.StatusCreated,
		CreatedAt:  time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC),
	}
	repository.shipments[shipment.ShipmentID] = shipment
	handler := NewHandler(repository)

	tests := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{name: "existing shipment", id: shipment.ShipmentID, wantStatus: http.StatusOK},
		{name: "unknown shipment", id: "123e4567-e89b-12d3-a456-426614174001", wantStatus: http.StatusNotFound},
		{name: "invalid id", id: "not-an-id", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, "/shipments/"+tt.id, "")
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
