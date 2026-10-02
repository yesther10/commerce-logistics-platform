package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
)

type Handler struct {
	repository domain.Repository
}

type createShipmentRequest struct {
	OrderID string `json:"order_id"`
}

func NewHandler(repository domain.Repository) http.Handler {
	handler := &Handler{repository: repository}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.healthz)
	mux.HandleFunc("POST /shipments", handler.createShipment)
	mux.HandleFunc("GET /shipments/{id}", handler.getShipment)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createShipment(w http.ResponseWriter, r *http.Request) {
	var request createShipmentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON with an order_id")
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	shipment, err := domain.NewShipment(request.OrderID, time.Now())
	if errors.Is(err, domain.ErrInvalidOrderID) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create shipment")
		return
	}

	if err := h.repository.Create(r.Context(), shipment); err != nil {
		writeError(w, http.StatusInternalServerError, "could not persist shipment")
		return
	}

	w.Header().Set("Location", "/shipments/"+shipment.ShipmentID)
	writeJSON(w, http.StatusCreated, shipment)
}

func (h *Handler) getShipment(w http.ResponseWriter, r *http.Request) {
	shipmentID := r.PathValue("id")
	if !domain.IsValidID(shipmentID) {
		writeError(w, http.StatusNotFound, domain.ErrNotFound.Error())
		return
	}

	shipment, err := h.repository.GetByID(r.Context(), shipmentID)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read shipment")
		return
	}

	writeJSON(w, http.StatusOK, shipment)
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("more than one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(message)})
}
