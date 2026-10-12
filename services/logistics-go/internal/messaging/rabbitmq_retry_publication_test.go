package messaging

import (
	"context"
	"testing"
	"time"
)

func TestPublisherRejectsInvalidRetryPublicationBeforeBrokerAccess(t *testing.T) {
	cases := []struct {
		name  string
		count int
		delay time.Duration
	}{
		{"zero retry count", 0, time.Second},
		{"retry count beyond topology", RetryQueueCount + 1, time.Second},
		{"zero delay", 1, 0},
		{"fractional millisecond delay", 1, 1500 * time.Microsecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			publisher := &Publisher{}
			err := publisher.PublishRetry(context.Background(), RetryMessage{RetryCount: tc.count, Delay: tc.delay})
			if err == nil {
				t.Fatal("PublishRetry() accepted invalid publication")
			}
		})
	}
}
