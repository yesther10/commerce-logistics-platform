package application

import (
	"context"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

type OutboxStore interface {
	PendingOutbox(ctx context.Context, limit int) ([]events.Envelope, error)
	MarkOutboxPublished(ctx context.Context, eventID string) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event events.Envelope) error
}

func PublishPending(ctx context.Context, store OutboxStore, publisher EventPublisher, limit int) (int, error) {
	eventsToPublish, err := store.PendingOutbox(ctx, limit)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, event := range eventsToPublish {
		if err := publisher.Publish(ctx, event); err != nil {
			return published, err
		}
		if err := store.MarkOutboxPublished(ctx, event.EventID); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}
