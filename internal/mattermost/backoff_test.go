package mattermost

import (
	"testing"
	"time"
)

func TestBackoffNextAndCap(t *testing.T) {
	t.Parallel()
	b := NewBackoff()
	if got := b.Next(); got != time.Second {
		t.Fatalf("first = %v", got)
	}
	if got := b.Next(); got != 2*time.Second {
		t.Fatalf("second = %v", got)
	}
	for i := 0; i < 10; i++ {
		b.Next()
	}
	if b.wait != 60*time.Second {
		t.Fatalf("cap wait = %v", b.wait)
	}
}

func TestBackoffReset(t *testing.T) {
	t.Parallel()
	b := NewBackoff()
	b.Next()
	b.Next()
	b.Reset()
	if got := b.Next(); got != time.Second {
		t.Fatalf("after reset = %v", got)
	}
}

func TestBackoffRecordDisconnect(t *testing.T) {
	t.Parallel()
	b := NewBackoff()
	b.Next()
	b.Next()
	b.RecordDisconnect(30 * time.Second)
	if b.wait == time.Second {
		t.Fatal("short disconnect should not reset backoff")
	}
	b.RecordDisconnect(60 * time.Second)
	if b.wait != time.Second {
		t.Fatalf("stable disconnect should reset, wait=%v", b.wait)
	}
}
