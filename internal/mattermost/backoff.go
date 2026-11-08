package mattermost

import (
	"context"
	"time"
)

// DefaultStableBeforeReset is how long a websocket must stay up before backoff resets.
const DefaultStableBeforeReset = 60 * time.Second

// Backoff implements exponential backoff for websocket reconnects.
// Initial delay 1s, factor 2, cap 60s. Backoff resets only after a connection
// stayed up for at least StableBeforeReset (default 60s).
type Backoff struct {
	Initial           time.Duration
	Factor            float64
	Cap               time.Duration
	StableBeforeReset time.Duration
	wait              time.Duration
	sleep             func(context.Context, time.Duration) error
}

func NewBackoff() Backoff {
	return Backoff{
		Initial:           time.Second,
		Factor:            2,
		Cap:               60 * time.Second,
		StableBeforeReset: DefaultStableBeforeReset,
		wait:              time.Second,
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

func (b *Backoff) RecordDisconnect(connectionDuration time.Duration) {
	if b.stableBeforeReset() > 0 && connectionDuration >= b.stableBeforeReset() {
		b.Reset()
	}
}

func (b *Backoff) stableBeforeReset() time.Duration {
	if b.StableBeforeReset > 0 {
		return b.StableBeforeReset
	}
	return DefaultStableBeforeReset
}

func (b *Backoff) Sleep(ctx context.Context) error {
	d := b.Next()
	if b.sleep != nil {
		return b.sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
