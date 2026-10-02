package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/application"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/domain"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, shipment domain.Shipment) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO shipments (shipment_id, order_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, shipment.ShipmentID, shipment.OrderID, shipment.Status, shipment.CreatedAt, shipment.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, shipmentID string) (domain.Shipment, error) {
	if !domain.IsValidID(shipmentID) {
		return domain.Shipment{}, domain.ErrNotFound
	}

	var shipment domain.Shipment
	err := r.db.QueryRowContext(ctx, `
		SELECT shipment_id, order_id, status, created_at, updated_at
		FROM shipments
		WHERE shipment_id = $1
	`, shipmentID).Scan(
		&shipment.ShipmentID,
		&shipment.OrderID,
		&shipment.Status,
		&shipment.CreatedAt,
		&shipment.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return domain.Shipment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Shipment{}, err
	}
	return shipment, nil
}

func (r *PostgresRepository) WithTransaction(ctx context.Context, fn func(context.Context, application.Transaction) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin shipment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(ctx, &postgresTransaction{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit shipment transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) PendingOutbox(ctx context.Context, limit int) ([]events.Envelope, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_id, event_type, event_version, occurred_at, correlation_id, causation_id, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY occurred_at, event_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []events.Envelope
	for rows.Next() {
		var event events.Envelope
		var payload []byte
		var causationID *string
		if err := rows.Scan(&event.EventID, &event.EventType, &event.EventVersion, &event.OccurredAt, &event.CorrelationID, &causationID, &payload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &event.Payload); err != nil {
			return nil, fmt.Errorf("decode outbox payload: %w", err)
		}
		event.CausationID = causationID
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PostgresRepository) MarkOutboxPublished(ctx context.Context, eventID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events SET published_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE event_id = $1 AND published_at IS NULL
	`, eventID)
	return err
}

type postgresTransaction struct {
	tx *sql.Tx
}

func (t *postgresTransaction) MarkEventProcessed(ctx context.Context, event events.Envelope, processedAt time.Time) (bool, error) {
	result, err := t.tx.ExecContext(ctx, `
		INSERT INTO processed_events (event_id, event_type, processed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $3)
		ON CONFLICT (event_id) DO NOTHING
	`, event.EventID, event.EventType, processedAt)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (t *postgresTransaction) FindShipmentByOrderID(ctx context.Context, orderID string) (domain.Shipment, error) {
	var shipment domain.Shipment
	err := t.tx.QueryRowContext(ctx, `
		SELECT shipment_id, order_id, status, created_at, updated_at
		FROM shipments WHERE order_id = $1 FOR UPDATE
	`, orderID).Scan(&shipment.ShipmentID, &shipment.OrderID, &shipment.Status, &shipment.CreatedAt, &shipment.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Shipment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Shipment{}, err
	}
	return shipment, nil
}

func (t *postgresTransaction) CreateShipment(ctx context.Context, shipment domain.Shipment) error {
	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO shipments (shipment_id, order_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, shipment.ShipmentID, shipment.OrderID, shipment.Status, shipment.CreatedAt, shipment.UpdatedAt)
	return err
}

func (t *postgresTransaction) AddOutboxEvent(ctx context.Context, event events.Envelope) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `
		INSERT INTO outbox_events
		(event_id, event_type, event_version, occurred_at, correlation_id, causation_id, payload, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $4, $4)
	`, event.EventID, event.EventType, event.EventVersion, event.OccurredAt, event.CorrelationID, event.CausationID, payload)
	return err
}
