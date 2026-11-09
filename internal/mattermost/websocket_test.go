package mattermost

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const cancelObservationMax = time.Second

func stubResolveIdentity(context.Context, string, string, string) (BotIdentity, error) {
	return BotIdentity{UserID: "u1", Username: "relaybot"}, nil
}

func waitForTCPAccept(t *testing.T, accepted <-chan struct{}) {
	t.Helper()
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("TCP accept did not occur before cancel")
	}
}

func TestBotListenerRunReturnsWithinOneSecondAfterCancelOnStallAcceptServer(t *testing.T) {
	stall, err := newStallAcceptListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stall.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	l := &BotListener{
		BotID:     "bot-1",
		Identity:  BotIdentity{UserID: "u1", Username: "relaybot"},
		WsOrigin:  stall.WSOrigin(),
		Token:     "token",
		Logger:    slog.Default(),
		ResolveID: stubResolveIdentity,
	}
	done := make(chan error, 1)
	go func() {
		done <- l.Run(ctx)
	}()
	waitForTCPAccept(t, stall.accepted)
	cancelAt := time.Now()
	cancel()
	var errRun error
	select {
	case errRun = <-done:
	case <-time.After(cancelObservationMax):
		t.Fatalf("BotListener.Run did not return within %s after cancel", cancelObservationMax)
	}
	afterCancel := time.Since(cancelAt)
	if !errors.Is(errRun, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", errRun)
	}
	t.Logf("measurement: listener-run-after-cancel elapsed=%s (from cancel, bound %s)", afterCancel.Round(time.Millisecond), cancelObservationMax)
}

func TestDefaultDialWebSocketReturnsWhenCanceledDuringSDKDial(t *testing.T) {
	orig := dialWebSocketSyncFn
	t.Cleanup(func() { dialWebSocketSyncFn = orig })
	dialWebSocketSyncFn = func(string, string) (WebSocket, error) {
		time.Sleep(10 * time.Minute)
		return nil, errors.New("unreachable")
	}

	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := DefaultDialWebSocket(ctx, "ws://127.0.0.1:1", "token")
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	var err error
	select {
	case err = <-done:
	case <-time.After(cancelObservationMax):
		t.Fatalf("DefaultDialWebSocket did not return within %s", cancelObservationMax)
	}
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	t.Logf("measurement: dial-return-on-cancel elapsed=%s (observation bound %s)", elapsed.Round(time.Millisecond), cancelObservationMax)
}

func TestBotListenerRunReturnsWithinOneSecondWhenDialStalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &BotListener{
		BotID:    "bot-1",
		Identity: BotIdentity{UserID: "u1", Username: "relaybot"},
		WsOrigin: "ws://127.0.0.1:1",
		Token:    "token",
		Logger:   slog.Default(),
		ResolveID: func(context.Context, string, string, string) (BotIdentity, error) {
			return BotIdentity{UserID: "u1", Username: "relaybot"}, nil
		},
		Dial: func(ctx context.Context, origin, token string) (WebSocket, error) {
			_ = origin
			_ = token
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(10 * time.Minute):
				return &mockWS{}, nil
			}
		},
	}
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- l.Run(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	var err error
	select {
	case err = <-done:
	case <-time.After(cancelObservationMax):
		t.Fatalf("listener Run did not return within %s", cancelObservationMax)
	}
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	t.Logf("measurement: listener-run-cancel elapsed=%s", elapsed.Round(time.Millisecond))
}

func TestDefaultDialWebSocketClosesLateConnection(t *testing.T) {
	var closed atomic.Int32
	orig := dialWebSocketSyncFn
	t.Cleanup(func() { dialWebSocketSyncFn = orig })
	dialWebSocketSyncFn = func(string, string) (WebSocket, error) {
		time.Sleep(200 * time.Millisecond)
		return &trackingWebSocket{onClose: func() { closed.Add(1) }}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := DefaultDialWebSocket(ctx, "ws://127.0.0.1:1", "token")
		errCh <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	var err error
	select {
	case err = <-errCh:
	case <-time.After(cancelObservationMax):
		t.Fatal("dial did not return after cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for closed.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if closed.Load() != 1 {
		t.Fatalf("late ws Close calls = %d, want 1", closed.Load())
	}
}

func TestDefaultDialWebSocketHandoffCancelRaceCloseExactlyOnce(t *testing.T) {
	orig := dialWebSocketSyncFn
	t.Cleanup(func() { dialWebSocketSyncFn = orig })

	var closed atomic.Int32
	gate := make(chan struct{})
	listenerWaiting := make(chan struct{})
	var waitingOnce sync.Once
	dialWebSocketSyncFn = func(string, string) (WebSocket, error) {
		waitingOnce.Do(func() { close(listenerWaiting) })
		<-gate
		return &trackingWebSocket{onClose: func() { closed.Add(1) }}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := DefaultDialWebSocket(ctx, "ws://127.0.0.1:1", "token")
		errCh <- err
	}()

	select {
	case <-listenerWaiting:
	case <-time.After(cancelObservationMax):
		t.Fatal("dial goroutine did not start sync")
	}
	// Caller blocked in select; sync blocked on gate — handoff has not started.
	var handoffOnce sync.Once
	go func() {
		handoffOnce.Do(func() {
			close(gate)
			cancel()
		})
	}()
	var err error
	select {
	case err = <-errCh:
	case <-time.After(cancelObservationMax):
		t.Fatal("dial did not return")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for closed.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := closed.Load(); n != 1 {
		t.Fatalf("Close calls = %d, want 1", n)
	}
}

type trackingWebSocket struct {
	onClose func()
}

func (t *trackingWebSocket) Listen()                           {}
func (t *trackingWebSocket) Close()                            { t.onClose() }
func (t *trackingWebSocket) EventChannel() <-chan *model.WebSocketEvent {
	return make(chan *model.WebSocketEvent)
}
func (t *trackingWebSocket) PingTimeoutChannel() <-chan bool { return make(chan bool) }
