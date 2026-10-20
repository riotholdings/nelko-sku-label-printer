package sku

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStockPause(t *testing.T) {
	for _, tc := range []struct {
		requested time.Duration
		height    float64
		want      time.Duration
	}{
		{0, 40, 4 * time.Second},
		{time.Second, 30, 4 * time.Second},
		{4 * time.Second, 75, 5750 * time.Millisecond},
		{10 * time.Second, 40, 10 * time.Second},
	} {
		if got := PrintPause(tc.requested, tc.height); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
func TestWaitAfterLastLabel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sent, err := Run(ctx, []Label{{Date: "2026-10-20", Sequence: 1}}, 1, time.Hour,
		func(Label) error { return nil }, func(int, int) { cancel() })
	if sent != 1 || !errors.Is(err, context.Canceled) {
		t.Fatal("final-label cooldown skipped", sent, err)
	}
}
