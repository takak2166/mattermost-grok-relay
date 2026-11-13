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

	"github.com/takak2166/mattermost-grok-relay/internal/config"
)

func testAllowedUserIDs(ids ...string) config.AllowedUserSet {
	set := make(config.AllowedUserSet, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

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

	dial := func(_ context.Context, _ string, _ string) (WebSocket, error) {
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
		ResolveID: func(context.Context, string, string, string) (BotIdentity, error) {
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
		AllowedSenders: testAllowedUserIDs("user-2"),
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
		ResolveID: func(context.Context, string, string, string) (BotIdentity, error) {
			return BotIdentity{UserID: "bot-1", Username: "relaybot"}, nil
		},
		Dial:    func(context.Context, string, string) (WebSocket, error) { return ws, nil },
		Backoff: b,
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

func TestBotListenerRefreshesIdentityBeforeEachConnect(t *testing.T) {
	var resolves atomic.Int32
	var dials atomic.Int32
	firstWS := &mockWS{events: make(chan *model.WebSocketEvent), ping: make(chan bool, 1)}
	secondWS := &mockWS{events: make(chan *model.WebSocketEvent), ping: make(chan bool, 1)}
	accepted := make(chan string, 2)

	l := &BotListener{
		BotID:    "bot-1",
		SiteURL:  "https://example.invalid",
		WsOrigin: "wss://example.invalid",
		Token:    "token",
		ResolveID: func(context.Context, string, string, string) (BotIdentity, error) {
			n := resolves.Add(1)
			if n == 1 {
				return BotIdentity{UserID: "u1", Username: "relaybot"}, nil
			}
			return BotIdentity{UserID: "u1", Username: "helperbot"}, nil
		},
		Dial: func(context.Context, string, string) (WebSocket, error) {
			n := dials.Add(1)
			if n == 1 {
				close(firstWS.events)
				return firstWS, nil
			}
			return secondWS, nil
		},
		Backoff: Backoff{
			Initial:           5 * time.Millisecond,
			Factor:            1,
			Cap:               5 * time.Millisecond,
			StableBeforeReset: time.Minute,
			wait:              5 * time.Millisecond,
		},
		OnPost: func(_ context.Context, _ string, post *model.Post) {
			select {
			case accepted <- post.Id:
			default:
			}
		},
		AllowedSenders: testAllowedUserIDs("user-2"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = l.Run(ctx) }()

	post := makePostedEvent(string(model.ChannelTypeOpen), []string{"u1"}, "user-2")
	postData, _ := post.GetData()["post"].(string)
	var p model.Post
	_ = json.Unmarshal([]byte(postData), &p)
	p.Message = "hi @helperbot"
	raw, _ := json.Marshal(&p)
	post.Add("post", string(raw))
	secondWS.events <- post

	select {
	case id := <-accepted:
		if id != "p1" {
			t.Fatalf("post id = %q", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for post after username refresh")
	}
	if resolves.Load() < 2 {
		t.Fatalf("resolve calls = %d, want at least 2", resolves.Load())
	}
	if l.Identity.Username != "helperbot" {
		t.Fatalf("identity username = %q, want helperbot", l.Identity.Username)
	}
	cancel()
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
		ResolveID: func(context.Context, string, string, string) (BotIdentity, error) {
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
		Dial: func(context.Context, string, string) (WebSocket, error) {
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
		ResolveID: func(context.Context, string, string, string) (BotIdentity, error) {
			return BotIdentity{UserID: "bot-config-2", Username: "relaybot"}, nil
		},
		Dial: func(context.Context, string, string) (WebSocket, error) {
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

func TestBotListenerNoWarnOnNonPostedEvent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ev := model.NewWebSocketEvent(model.WebsocketEventTyping, "", "", "", nil, "")

	l := &BotListener{
		BotID:  "bot-1",
		Logger: logger,
	}
	l.handleEvent(context.Background(), ev)
	if strings.Contains(buf.String(), failClassPostedEventInvalid) {
		t.Fatalf("unexpected warn for non-posted event: %s", buf.String())
	}
}

func TestBotListenerLogsInvalidPostedEventWithoutPayload(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	sensitivePost := `not-json-body-with-fake-token-value`
	ev := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "", "", nil, "")
	ev.Add("post", sensitivePost)

	l := &BotListener{
		BotID:    "bot-1",
		Logger:   logger,
		Identity: BotIdentity{UserID: "bot-1", Username: "relaybot"},
	}
	l.handleEvent(context.Background(), ev)

	out := buf.String()
	if strings.Contains(out, "fake-token") || strings.Contains(out, sensitivePost) {
		t.Fatalf("log leaked payload: %s", out)
	}
	if !strings.Contains(out, failClassPostedEventInvalid) {
		t.Fatalf("expected fail_class in log: %s", out)
	}
}

func TestBotListenerDeniedSenderLogsWithoutOnPost(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	called := make(chan struct{}, 1)

	post := makePostedEvent(string(model.ChannelTypeDirect), nil, "intruder-1")
	post.Add("sender_name", "@someone")
	postData, _ := post.GetData()["post"].(string)
	var p model.Post
	_ = json.Unmarshal([]byte(postData), &p)
	p.ChannelId = "ch-deny-1"
	p.Message = "secret-free body"
	raw, _ := json.Marshal(&p)
	post.Add("post", string(raw))

	l := &BotListener{
		BotID:          "bot-1",
		Logger:         logger,
		Identity:       BotIdentity{UserID: "bot-1", Username: "relaybot"},
		AllowedSenders: testAllowedUserIDs("allowed-user-1"),
		OnPost: func(context.Context, string, *model.Post) {
			called <- struct{}{}
		},
	}
	l.handleEvent(context.Background(), post)

	select {
	case <-called:
		t.Fatal("OnPost must not run for disallowed sender")
	default:
	}

	out := buf.String()
	for _, want := range []string{
		logMsgRelaySenderNotAllowed,
		failClassSenderNotAllowed,
		`"level":"INFO"`,
		`"sender_user_id":"intruder-1"`,
		`"sender_name":"@someone"`,
		`"channel_type":"D"`,
		`"channel_id":"ch-deny-1"`,
		`"post_id":"p1"`,
		`"message_prefix":"secret-free body"`,
		`"bot_id":"bot-1"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %q: %s", want, out)
		}
	}
	if strings.Count(out, logMsgRelaySenderNotAllowed) != 1 {
		t.Fatalf("expected exactly one deny log, got: %s", out)
	}
}

func TestBotListenerNilAllowedSendersDeniesOnPost(t *testing.T) {
	t.Parallel()
	called := make(chan struct{}, 1)
	post := makePostedEvent(string(model.ChannelTypeDirect), nil, "user-x")

	l := &BotListener{
		BotID:    "bot-1",
		Identity: BotIdentity{UserID: "bot-1", Username: "relaybot"},
		OnPost: func(context.Context, string, *model.Post) {
			called <- struct{}{}
		},
	}
	l.handleEvent(context.Background(), post)

	select {
	case <-called:
		t.Fatal("OnPost must not run when AllowedSenders is unset")
	default:
	}
}

func TestBotListenerAllowedSenderInvokesOnPost(t *testing.T) {
	t.Parallel()
	called := make(chan string, 1)
	post := makePostedEvent(string(model.ChannelTypeDirect), nil, "allowed-user-1")

	l := &BotListener{
		BotID:          "bot-1",
		Identity:       BotIdentity{UserID: "bot-1", Username: "relaybot"},
		AllowedSenders: testAllowedUserIDs("allowed-user-1"),
		OnPost: func(_ context.Context, _ string, p *model.Post) {
			called <- p.Id
		},
	}
	l.handleEvent(context.Background(), post)

	select {
	case id := <-called:
		if id != "p1" {
			t.Fatalf("post id = %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for OnPost")
	}
}
