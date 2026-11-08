package mattermost

import (
	"context"
	"time"
)

// Backoff implements exponential backoff for websocket reconnects.
// Initial delay 1s, factor 2, cap 60s; reset after a successful connection.
type Backoff struct {
	Initial time.Duration
	Factor  float64
	Cap     time.Duration
	wait    time.Duration
}

func NewBackoff() Backoff {
	return Backoff{
		Initial: time.Second,
		Factor:  2,
		Cap:     60 * time.Second,
		wait:    time.Second,
	}
}

func (b *Backoff) Reset() {
	b.wait = b.Initial
}

func (b *Backoff) Next() time.Duration {
	d := b.wait
	b.wait = time.Duration(float64(b.wait) * b.Factor)
	if b.wait > b.Cap {
		b.wait = b.Cap
	}
	return d
}

func (b *Backoff) Sleep(ctx context.Context) error {
	timer := time.NewTimer(b.Next())
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
