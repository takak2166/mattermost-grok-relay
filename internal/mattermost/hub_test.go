package mattermost

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/takak2166/mattermost-grok-relay/internal/config"
)

func TestBotListenerExitClass(t *testing.T) {
	t.Parallel()
	if log, class := botListenerExitClass(context.Canceled); log || class != "" {
		t.Fatalf("canceled: log=%v class=%q", log, class)
	}
	if log, class := botListenerExitClass(errors.Join(context.Canceled, errors.New("ignored"))); log || class != "" {
		t.Fatalf("joined canceled: log=%v class=%q", log, class)
	}
	if log, class := botListenerExitClass(context.DeadlineExceeded); log || class != "" {
		t.Fatalf("deadline exceeded: log=%v class=%q", log, class)
	}
	log, class := botListenerExitClass(errors.New("standalone"))
	if !log || class != exitClassListenerExited {
		t.Fatalf("abnormal: log=%v class=%q", log, class)
	}
}

func TestLogBotListenerExitOmitsSensitiveErrorText(t *testing.T) {
	t.Parallel()
	sensitive := errors.New("https://fake.example.invalid/subpath wss://fake?token=fake-bot-token-value")
	if log, class := botListenerExitClass(sensitive); !log || class != exitClassListenerExited {
		t.Fatalf("expected abnormal exit class, got log=%v class=%q", log, class)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logBotListenerExit(logger, "bot-config-id", exitClassListenerExited)

	out := buf.String()
	for _, forbidden := range []string{
		"https://",
		"wss://",
		"fake-bot-token",
		"token=",
		"standalone",
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("log must not contain %q: %s", forbidden, out)
		}
	}
	for _, want := range []string{"bot-config-id", exitClassListenerExited, "mattermost bot listener stopped"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %q: %s", want, out)
		}
	}
}

func TestHubRunLogsAbnormalListenerExit(t *testing.T) {
	t.Parallel()
	sensitive := errors.New("https://fake.example.invalid api token fake-value")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := &Hub{
		SiteURL: "https://fake.example.invalid",
		Bots:    []config.Bot{{ID: "bot-1"}},
		Logger:  logger,
		runListener: func(context.Context, *BotListener) error {
			return sensitive
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = h.Run(ctx)

	out := buf.String()
	if strings.Contains(out, "https://") || strings.Contains(out, "fake-value") || strings.Contains(out, "api token") {
		t.Fatalf("hub log leaked sensitive text: %s", out)
	}
	if !strings.Contains(out, `"bot_id":"bot-1"`) {
		t.Fatalf("expected bot_id in log: %s", out)
	}
	if !strings.Contains(out, exitClassListenerExited) {
		t.Fatalf("expected exit_class in log: %s", out)
	}
}
