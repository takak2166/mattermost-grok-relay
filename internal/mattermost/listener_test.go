package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
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

func makePostedEvent(channelType string, mentionIDs []string, userID string) *model.WebSocketEvent {
	post := &model.Post{Id: "p1", UserId: userID, Message: "hi"}
	raw, _ := json.Marshal(post)
	mentions, _ := json.Marshal(mentionIDs)
	ev := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "", "", nil, "")
	ev.Add("channel_type", channelType)
	ev.Add("post", string(raw))
	ev.Add("mentions", string(mentions))
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
		WsOrigin: "wss://example.invalid",
		Token:    "token",
		Dial:     dial,
		ResolveID: func(_, _, _ string) (BotIdentity, error) {
			return BotIdentity{UserID: "bot-1", Username: "relaybot"}, nil
		},
		Backoff: Backoff{
			Initial:           10 * time.Millisecond,
			Factor:            2,
			Cap:               20 * time.Millisecond,
			StableBeforeReset: time.Minute,
			wait:              10 * time.Millisecond,
		},
		OnPost: func(_ context.Context, botID string, post *model.Post) {
			accepted <- post.Id
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = l.Run(ctx)
	}()

	post := makePostedEvent(string(model.ChannelTypeOpen), []string{"bot-1"}, "user-2")
	postData, _ := post.GetData()["post"].(string)
	var p model.Post
	_ = json.Unmarshal([]byte(postData), &p)
	p.Message = "hi @relaybot"
	raw, _ := json.Marshal(&p)
	post.Add("post", string(raw))
	secondWS.events <- post
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

func TestBotListenerFlapDoesNotResetBackoffImmediately(t *testing.T) {
	t.Parallel()
	var slept []time.Duration
	b := NewBackoff()
	b.StableBeforeReset = time.Minute
	b.wait = time.Second
	b.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	ws := &mockWS{events: make(chan *model.WebSocketEvent), ping: make(chan bool, 1)}
	close(ws.events)

	l := &BotListener{
		BotID:    "bot-1",
		Identity: BotIdentity{UserID: "bot-1"},
		WsOrigin: "wss://example.invalid",
		Token:    "token",
		Dial:     func(_, _ string) (WebSocket, error) { return ws, nil },
		Backoff:  b,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = l.Run(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	if len(slept) == 0 {
		t.Fatal("expected backoff sleep after flap")
	}
	if slept[0] != time.Second {
		t.Fatalf("first sleep = %v, want 1s (not reset after short session)", slept[0])
	}
}

func TestBotListenerIdentityFailureLogsRedacted(t *testing.T) {
	t.Parallel()
	sensitive := fmt.Errorf("mattermost: get me: %w", &url.Error{
		Op:  "Get",
		URL: "https://fake.example.invalid/api/v4/users/me",
		Err: errors.New("authorization bearer fake-bot-token-value"),
	})

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	l := &BotListener{
		BotID:   "bot-config-1",
		SiteURL: "https://fake.example.invalid",
		Logger:  logger,
		ResolveID: func(_, _, _ string) (BotIdentity, error) {
			return BotIdentity{}, sensitive
		},
		Backoff: Backoff{
			Initial:           time.Millisecond,
			Factor:            1,
			Cap:               time.Millisecond,
			StableBeforeReset: time.Minute,
			wait:              time.Millisecond,
			sleep:             func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		},
		Dial: func(_, _ string) (WebSocket, error) {
			t.Fatal("dial must not run before identity resolves")
			return nil, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = l.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	out := buf.String()
	for _, forbidden := range []string{"https://", "fake-bot-token", "bearer", "users/me"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("identity failure log leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, failClassIdentityTransport) {
		t.Fatalf("expected fail_class in log: %s", out)
	}
}

func TestBotListenerDialFailureLogsRedacted(t *testing.T) {
	t.Parallel()
	sensitive := fmt.Errorf("mattermost: websocket dial: %w", &url.Error{
		Op:  "GET",
		URL: "wss://fake.example.invalid/api/v4/websocket",
		Err: errors.New("unexpected EOF"),
	})

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	var dials atomic.Int32

	l := &BotListener{
		BotID:    "bot-config-2",
		Identity: BotIdentity{UserID: "bot-config-2", Username: "relaybot"},
		Logger:   logger,
		Dial: func(_, _ string) (WebSocket, error) {
			dials.Add(1)
			return nil, sensitive
		},
		Backoff: Backoff{
			Initial:           time.Millisecond,
			Factor:            1,
			Cap:               time.Millisecond,
			StableBeforeReset: time.Minute,
			wait:              time.Millisecond,
			sleep:             func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = l.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if dials.Load() < 1 {
		t.Fatal("expected at least one dial attempt")
	}
	out := buf.String()
	for _, forbidden := range []string{"wss://", "fake.example", "unexpected EOF"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("dial failure log leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, failClassWebSocketTransport) {
		t.Fatalf("expected fail_class in log: %s", out)
	}
}
