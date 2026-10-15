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
			name:     "no args",
			args:     []string{"mattermost-grok-relay"},
			wantCode: 1,
			wantStderr: "relay logic is not implemented yet; use -version to print the build version\n",
		},
		{
			name:       "unknown flag",
			args:       []string{"mattermost-grok-relay", "-unknown"},
			wantCode:   1,
			wantStderr: "flag provided but not defined: -unknown",
		},
		{
			name:     "positional args",
			args:     []string{"mattermost-grok-relay", "extra"},
			wantCode: 1,
			wantStderr: "unknown arguments:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
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
