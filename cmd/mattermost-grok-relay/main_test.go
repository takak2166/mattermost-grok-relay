package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/takak2166/mattermost-grok-relay/internal/app"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
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
			name: "config loaded then relay placeholder",
			args: []string{
				"mattermost-grok-relay",
				"-config", "testdata/minimal.json",
			},
			wantCode:   1,
			wantStderr: "relay logic is not implemented yet",
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
				if key == "RELAY_TEST_TOKEN" {
					return "test-token-value", true
				}
				return "", false
			}
			code := run(tt.args, &stdout, &stderr, lookup)
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
