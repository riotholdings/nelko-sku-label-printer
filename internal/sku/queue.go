package sku

import (
	"context"
	"fmt"
	"time"
)

// Run sends each physical label separately. A failed/uncertain send is never
// retried automatically, because it may already have reached the printer.
// sent counts transmissions accepted by the transport, not verified paper output.
func Run(ctx context.Context, labels []Label, copies int, delay time.Duration, send func(Label) error, progress func(int, int)) (sent int, err error) {
	if len(labels) == 0 || copies < 1 || copies > 100 || len(labels) > MaxBatch/copies {
		return 0, fmt.Errorf("choose 1-100 copies and at most %d physical labels per job", MaxBatch)
	}
	if delay < 0 {
		return 0, fmt.Errorf("delay cannot be negative")
	}
	total := len(labels) * copies
	for _, l := range labels {
		for c := 0; c < copies; c++ {
			if err := ctx.Err(); err != nil {
				return sent, err
			}
			if err := send(l); err != nil {
				return sent, fmt.Errorf("send %s (copy %d): %w; inspect that label before retrying", l.String(), c+1, err)
			}
			sent++
			if progress != nil {
				progress(sent, total)
			}
			if sent < total && delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return sent, ctx.Err()
				case <-timer.C:
				}
			}
		}
	}
	return sent, nil
}
