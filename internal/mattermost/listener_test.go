package mattermost

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

type mockWS struct {
	events chan *model.WebSocketEvent
	ping   chan bool
	closed int32
}

func (m *mockWS) Listen() {}
func (m *mockWS) Close()  { atomic.StoreInt32(&m.closed, 1) }
func (m *mockWS) EventChannel() <-chan *model.WebSocketEvent {
	return m.events
}
func (m *mockWS) PingTimeoutChannel() <-chan bool { return m.ping }

func makePostedEvent(channelType, message, userID string) *model.WebSocketEvent {
	post := &model.Post{Id: "p1", UserId: userID, Message: message}
	raw, _ := json.Marshal(post)
	ev := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "", "", nil, "")
	ev.Add("channel_type", channelType)
	ev.Add("post", string(raw))
	return ev
}

func TestBotListenerReconnectBackoff(t *testing.T) {
	t.Parallel()
	var dials atomic.Int32

	firstWS := &mockWS{events: make(chan *model.WebSocketEvent), ping: make(chan bool, 1)}
	secondWS := &mockWS{events: make(chan *model.WebSocketEvent), ping: make(chan bool, 1)}

	dial := func(_, _ string) (WebSocket, error) {
		n := dials.Add(1)
		if n == 1 {
			close(firstWS.events)
			return firstWS, nil
		}
		if n == 2 {
			return nil, errors.New("dial failed")
		}
		return secondWS, nil
	}

	accepted := make(chan string, 1)
	l := &BotListener{
		BotID:    "bot-1",
		Identity: BotIdentity{UserID: "bot-1", Username: "relaybot"},
		SiteURL:  "https://example.invalid",
		WsOrigin: "ws://example.invalid",
		Token:    "token",
		Dial:     dial,
		ResolveID: func(_, _, _ string) (BotIdentity, error) {
			return BotIdentity{UserID: "bot-1", Username: "relaybot"}, nil
		},
		Backoff: Backoff{Initial: 10 * time.Millisecond, Factor: 2, Cap: 20 * time.Millisecond, wait: 10 * time.Millisecond},
		OnPost: func(_ context.Context, botID string, post *model.Post) {
			accepted <- post.Id
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = l.Run(ctx)
	}()

	secondWS.events <- makePostedEvent(string(model.ChannelTypeOpen), "hi @relaybot", "user-2")
	select {
	case id := <-accepted:
		if id != "p1" {
			t.Fatalf("post id = %q", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for accepted post")
	}

	if dials.Load() < 2 {
		t.Fatalf("expected at least 2 dials, got %d", dials.Load())
	}
	cancel()
}
