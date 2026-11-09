package mattermost

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mattermost/mattermost/server/public/model"
)

const defaultDialTimeout = 30 * time.Second

// WebSocket abstracts the official Mattermost WebSocket client for testing.
type WebSocket interface {
	Listen()
	Close()
	EventChannel() <-chan *model.WebSocketEvent
	PingTimeoutChannel() <-chan bool
}

type liveWebSocket struct {
	inner *model.WebSocketClient
}

func (w *liveWebSocket) Listen()                                    { w.inner.Listen() }
func (w *liveWebSocket) Close()                                     { w.inner.Close() }
func (w *liveWebSocket) EventChannel() <-chan *model.WebSocketEvent { return w.inner.EventChannel }
func (w *liveWebSocket) PingTimeoutChannel() <-chan bool            { return w.inner.PingTimeoutChannel }

// DialWebSocket opens a WebSocket using the official Mattermost client (server/public v0.6.0).
type DialWebSocket func(ctx context.Context, wsOrigin, token string) (WebSocket, error)

func newWebSocketDialer(timeout time.Duration) *websocket.Dialer {
	netDialer := &net.Dialer{Timeout: timeout}
	return &websocket.Dialer{
		HandshakeTimeout: timeout,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return netDialer.DialContext(ctx, network, addr)
		},
	}
}

// dialWebSocketSyncFn is the blocking SDK dial used by DefaultDialWebSocket (overridable in tests).
var dialWebSocketSyncFn = dialWebSocketSync

func dialWebSocketSync(wsOrigin, token string) (WebSocket, error) {
	dialer := newWebSocketDialer(defaultDialTimeout)
	client, err := model.NewWebSocketClient4WithDialer(dialer, wsOrigin, token)
	if err != nil {
		return nil, err
	}
	return &liveWebSocket{inner: client}, nil
}

// DefaultDialWebSocket runs the blocking SDK dial in a goroutine. The caller returns
// ctx.Err() as soon as ctx is done without waiting for the SDK dial to finish.
// A connection that completes after ctx is canceled is closed by the dial goroutine.
func DefaultDialWebSocket(ctx context.Context, wsOrigin, token string) (WebSocket, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type dialResult struct {
		ws  WebSocket
		err error
	}
	ch := make(chan dialResult, 1)
	go func() {
		ws, err := dialWebSocketSyncFn(wsOrigin, token)
		if ctx.Err() != nil {
			if ws != nil {
				ws.Close()
			}
			return
		}
		res := dialResult{ws: ws, err: err}
		select {
		case ch <- res:
		case <-ctx.Done():
			if ws != nil {
				ws.Close()
			}
		}
	}()
	select {
	case <-ctx.Done():
		select {
		case r := <-ch:
			if r.ws != nil {
				r.ws.Close()
			}
		default:
		}
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			if r.ws != nil {
				r.ws.Close()
			}
			return nil, fmt.Errorf("mattermost: websocket dial: %w", r.err)
		}
		return r.ws, nil
	}
}
