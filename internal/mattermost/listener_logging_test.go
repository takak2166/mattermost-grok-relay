package mattermost

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestClassifyIdentityResolveFail(t *testing.T) {
	t.Parallel()
	transport := fmt.Errorf("wrap: %w", &url.Error{
		Op:  "Get",
		URL: "https://fake.example.invalid/api/v4/users/me",
		Err: errors.New("connection refused"),
	})
	if got := classifyIdentityResolveFail(transport); got != failClassIdentityTransport {
		t.Fatalf("transport: got %q want %q", got, failClassIdentityTransport)
	}
	api := model.NewAppError("GetMe", "api.user.get_me.app_error", nil, "", 500)
	if got := classifyIdentityResolveFail(api); got != failClassIdentityAPI {
		t.Fatalf("api: got %q want %q", got, failClassIdentityAPI)
	}
	generic := errors.New("https://fake.example.invalid token=fake")
	if got := classifyIdentityResolveFail(generic); got != failClassIdentityResolve {
		t.Fatalf("generic: got %q want %q", got, failClassIdentityResolve)
	}
}

func TestClassifyWebSocketDialFail(t *testing.T) {
	t.Parallel()
	transport := fmt.Errorf("dial: %w", &url.Error{
		Op:  "GET",
		URL: "wss://fake.example.invalid/api/v4/websocket",
		Err: errors.New("tls: bad cert"),
	})
	if got := classifyWebSocketDialFail(transport); got != failClassWebSocketTransport {
		t.Fatalf("transport: got %q want %q", got, failClassWebSocketTransport)
	}
	inner := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	wrapped := model.NewAppError("NewWebSocketClient", "model.websocket_client.connect_fail.app_error", nil, "", 500).Wrap(inner)
	outer := fmt.Errorf("mattermost: websocket dial: %w", wrapped)
	if got := classifyWebSocketDialFail(outer); got != failClassWebSocketTransport {
		t.Fatalf("wrapped dial: got %q want %q", got, failClassWebSocketTransport)
	}
}

func TestLogBotListenerWarnOmitsErrorText(t *testing.T) {
	t.Parallel()
	sensitive := &url.Error{
		Op:  "Get",
		URL: "https://fake.example.invalid/api/v4/users/me?token=fake-secret",
		Err: errors.New("dial tcp: lookup fake"),
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logBotListenerWarn(logger, "bot-a", "mattermost resolve bot identity failed", classifyIdentityResolveFail(sensitive))
	logBotListenerWarn(logger, "bot-a", "mattermost websocket connect failed", classifyWebSocketDialFail(sensitive))

	out := buf.String()
	for _, forbidden := range []string{
		"https://",
		"wss://",
		"fake-secret",
		"token=",
		"dial tcp",
		"users/me",
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, out)
		}
	}
	for _, want := range []string{
		"bot-a",
		failClassIdentityTransport,
		failClassWebSocketTransport,
		"mattermost resolve bot identity failed",
		"mattermost websocket connect failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %q: %s", want, out)
		}
	}
}
