package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/takak2166/mattermost-grok-relay/internal/app"
	"github.com/takak2166/mattermost-grok-relay/internal/config"
	"github.com/takak2166/mattermost-grok-relay/internal/mattermost"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		wantCode        int
		wantStdout      string
		wantStderr      string
		assertNoWebhook string
	}{
		{
			name:       "version flag",
			args:       []string{"mattermost-grok-relay", "-version"},
			wantCode:   0,
			wantStdout: "mattermost-grok-relay " + app.Version + "\n",
		},
		{
			name:       "no args without config",
			args:       []string{"mattermost-grok-relay"},
			wantCode:   1,
			wantStderr: "config load failed:",
		},
		{
			name: "config loaded without mattermost url",
			args: []string{
				"mattermost-grok-relay",
				"-config", "testdata/minimal.json",
			},
			wantCode:        1,
			wantStderr:      "RELAY_MATTERMOST_URL is required",
			assertNoWebhook: "https://grok.example.invalid/hook/test",
		},
		{
			name:       "unknown flag",
			args:       []string{"mattermost-grok-relay", "-unknown"},
			wantCode:   1,
			wantStderr: "flag provided but not defined: -unknown",
		},
		{
			name:       "positional args",
			args:       []string{"mattermost-grok-relay", "extra"},
			wantCode:   1,
			wantStderr: "unknown arguments:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			lookup := func(key string) (string, bool) {
				switch key {
				case "RELAY_TEST_TOKEN":
					return "test-token-value", true
				case "RELAY_TEST_WEBHOOK_URL":
					return "https://grok.example.invalid/hook/test", true
				default:
					return "", false
				}
			}
			code := run(tt.args, &stdout, &stderr, lookup)
			if tt.assertNoWebhook != "" {
				if strings.Contains(stdout.String(), tt.assertNoWebhook) || strings.Contains(stderr.String(), tt.assertNoWebhook) {
					t.Fatalf("output leaked webhook URL")
				}
			}
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && stdout.String() != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunWithRelayConfigJSONEnv(t *testing.T) {
	t.Setenv("RELAY_CONFIG_JSON", `{"bots":[{"id":"env-bot","url_env":"RELAY_TEST_WEBHOOK_URL","token_env":"RELAY_TEST_TOKEN"}]}`)

	var stdout, stderr bytes.Buffer
	lookup := func(key string) (string, bool) {
		switch key {
		case "RELAY_TEST_TOKEN":
			return "test-token-value", true
		case "RELAY_TEST_WEBHOOK_URL":
			return "https://grok.example.invalid/hook/test", true
		default:
			return os.LookupEnv(key)
		}
	}
	code := run([]string{"mattermost-grok-relay"}, &stdout, &stderr, lookup)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "RELAY_MATTERMOST_URL is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	leak := "https://grok.example.invalid/hook/test"
	if strings.Contains(stdout.String(), leak) || strings.Contains(stderr.String(), leak) {
		t.Fatalf("output leaked webhook URL")
	}
}

func relayLookup() func(string) (string, bool) {
	return func(key string) (string, bool) {
		switch key {
		case "RELAY_MATTERMOST_URL":
			return "https://fake.example.invalid", true
		case "RELAY_ALLOWED_USER_IDS":
			return "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", true
		case "RELAY_TEST_TOKEN":
			return "test-token-value", true
		case "RELAY_TEST_WEBHOOK_URL":
			return "https://grok.example.invalid/hook/test", true
		default:
			return "", false
		}
	}
}

func TestRunMissingAllowedUserIDs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	lookup := func(key string) (string, bool) {
		switch key {
		case "RELAY_MATTERMOST_URL":
			return "https://fake.example.invalid", true
		case "RELAY_TEST_TOKEN":
			return "test-token-value", true
		case "RELAY_TEST_WEBHOOK_URL":
			return "https://grok.example.invalid/hook/test", true
		default:
			return "", false
		}
	}
	code := run(
		[]string{"mattermost-grok-relay", "-config", "testdata/minimal.json"},
		&stdout, &stderr,
		lookup,
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "RELAY_ALLOWED_USER_IDS") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRelaySuccessExit0(t *testing.T) {
	old := startRelay
	t.Cleanup(func() { startRelay = old })
	startRelay = func(context.Context, string, config.Config, config.AllowedUserSet) error {
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"mattermost-grok-relay", "-config", "testdata/minimal.json"},
		&stdout, &stderr,
		relayLookup(),
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestRunRelayCanceledExit0(t *testing.T) {
	old := startRelay
	t.Cleanup(func() { startRelay = old })
	startRelay = func(context.Context, string, config.Config, config.AllowedUserSet) error {
		return context.Canceled
	}

	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"mattermost-grok-relay", "-config", "testdata/minimal.json"},
		&stdout, &stderr,
		relayLookup(),
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
}

func TestRunRelayAllListenersStoppedExit1(t *testing.T) {
	old := startRelay
	t.Cleanup(func() { startRelay = old })
	startRelay = func(context.Context, string, config.Config, config.AllowedUserSet) error {
		return mattermost.ErrAllListenersStopped
	}

	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"mattermost-grok-relay", "-config", "testdata/minimal.json"},
		&stdout, &stderr,
		relayLookup(),
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "relay stopped:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRelayAbnormalErrorExit1(t *testing.T) {
	old := startRelay
	t.Cleanup(func() { startRelay = old })
	startRelay = func(context.Context, string, config.Config, config.AllowedUserSet) error {
		return errors.New("abnormal relay failure")
	}

	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"mattermost-grok-relay", "-config", "testdata/minimal.json"},
		&stdout, &stderr,
		relayLookup(),
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
