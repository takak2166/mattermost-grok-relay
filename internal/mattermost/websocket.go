package mattermost

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

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

func (w *liveWebSocket) Listen() { w.inner.Listen() }
func (w *liveWebSocket) Close()  { w.inner.Close() }
func (w *liveWebSocket) EventChannel() <-chan *model.WebSocketEvent {
	return w.inner.EventChannel
}
func (w *liveWebSocket) PingTimeoutChannel() <-chan bool { return w.inner.PingTimeoutChannel }

// DialWebSocket opens a WebSocket using model.NewWebSocketClient4.
type DialWebSocket func(wsOrigin, token string) (WebSocket, error)

func DefaultDialWebSocket(wsOrigin, token string) (WebSocket, error) {
	client, err := model.NewWebSocketClient4(wsOrigin, token)
	if err != nil {
		return nil, fmt.Errorf("mattermost: websocket dial: %w", err)
	}
	return &liveWebSocket{inner: client}, nil
}
