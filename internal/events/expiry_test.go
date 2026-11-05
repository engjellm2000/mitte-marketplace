package events

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeExpirer struct {
	calls atomic.Int32
	batch atomic.Int32
}

func (f *fakeExpirer) ExpirePendingOrders(context.Context, int) (int, error) {
	f.calls.Add(1)
	return 0, nil
}

func TestPaymentExpiryWorkerRunsWithoutKafka(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	expirer := &fakeExpirer{}
	RunPaymentExpiryWorker(ctx, expirer, nil, time.Minute, 100)
	if got := expirer.calls.Load(); got != 1 {
		t.Fatalf("ExpirePendingOrders calls = %d, want 1 immediate sweep", got)
	}
}
