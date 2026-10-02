package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	StatusCreated    = "created"
	maxOrderIDLength = 255
)

var (
	ErrInvalidOrderID = errors.New("order_id must be a non-empty string of at most 255 characters")
	ErrNotFound       = errors.New("shipment not found")
)

type Shipment struct {
	ShipmentID string    `json:"shipment_id"`
	OrderID    string    `json:"order_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Repository interface {
	Create(ctx context.Context, shipment Shipment) error
	GetByID(ctx context.Context, shipmentID string) (Shipment, error)
}

func NewShipment(orderID string, now time.Time) (Shipment, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" || utf8.RuneCountInString(orderID) > maxOrderIDLength {
		return Shipment{}, ErrInvalidOrderID
	}

	shipmentID, err := NewID()
	if err != nil {
		return Shipment{}, fmt.Errorf("generate shipment id: %w", err)
	}
	return NewShipmentWithID(orderID, shipmentID, now)
}

func NewShipmentWithID(orderID, shipmentID string, now time.Time) (Shipment, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" || utf8.RuneCountInString(orderID) > maxOrderIDLength || !IsValidID(shipmentID) {
		return Shipment{}, ErrInvalidOrderID
	}

	now = now.UTC()
	return Shipment{
		ShipmentID: shipmentID,
		OrderID:    orderID,
		Status:     StatusCreated,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func IsValidID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}

	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16
}

func NewID() (string, error) {
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
