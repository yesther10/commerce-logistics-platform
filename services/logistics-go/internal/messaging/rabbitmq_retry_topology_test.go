package messaging

import (
	"strconv"
	"testing"
	"time"

	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/events"
)

func TestShipmentRequestedRetryRoutingKeys(t *testing.T) {
	for retryCount, want := range map[int]string{1: "logistics.shipment.requested.retry.1", 2: "logistics.shipment.requested.retry.2", 3: "logistics.shipment.requested.retry.3"} {
		t.Run(want, func(t *testing.T) {
			if got := events.ShipmentRequestedRetryRK(retryCount); got != want {
				t.Fatalf("routing key = %q, want %q", got, want)
			}
		})
	}
}

func TestValidateRetryQueuesUsesBrokerMillisecondGranularity(t *testing.T) {
	cases := []struct {
		name    string
		delays  []time.Duration
		wantErr bool
	}{
		{"strict whole milliseconds", []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}, false},
		{"wrong queue count", []time.Duration{time.Millisecond}, true},
		{"fractional millisecond", []time.Duration{1500 * time.Microsecond, 2 * time.Millisecond, 3 * time.Millisecond}, true},
		{"equal broker milliseconds", []time.Duration{time.Millisecond, 1000000 * time.Nanosecond, 2 * time.Millisecond}, true},
		{"not increasing", []time.Duration{time.Millisecond, time.Millisecond, 2 * time.Millisecond}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			queues := make([]RetryQueueConfig, len(tc.delays))
			for index, delay := range tc.delays {
				queues[index] = RetryQueueConfig{Queue: "retry." + strconv.Itoa(index+1), Delay: delay}
			}
			if err := validateRetryQueues(queues); (err != nil) != tc.wantErr {
				t.Fatalf("validateRetryQueues() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateRetryQueuesRejectsAnEmptyQueueName(t *testing.T) {
	queues := []RetryQueueConfig{{Queue: "retry.1", Delay: time.Second}, {Delay: 5 * time.Second}, {Queue: "retry.3", Delay: 15 * time.Second}}
	if err := validateRetryQueues(queues); err == nil {
		t.Fatal("validateRetryQueues() accepted an empty queue name")
	}
}
