package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

func TestLoadSuccessFromJSON(t *testing.T) {
	t.Parallel()

	cfg, err := Load(LoadOptions{
		ConfigJSON: `{"bots":[{"id":"orimar","webhook_url":"https://grok.example.invalid/hook/orimar","token_env":"MM_TOKEN_ORIMAR"}]}`,
		LookupEnv: func(key string) (string, bool) {
			if key == "MM_TOKEN_ORIMAR" {
				return testToken, true
			}
			return "", false
		},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Bots) != 1 {
		t.Fatalf("bots len = %d, want 1", len(cfg.Bots))
	}
	if cfg.Bots[0].ID != "orimar" {
		t.Fatalf("id = %q", cfg.Bots[0].ID)
	}
	if cfg.Bots[0].Token.Value() != testToken {
		t.Fatalf("token mismatch")
	}
	if cfg.Bots[0].WebhookURL.Value() != "https://grok.example.invalid/hook/orimar" {
		t.Fatalf("webhook mismatch")
	}
}

func TestLoadSuccessFromFileAndTokenFile(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"/etc/relay/config.json":  []byte(`{"bots":[{"id":"beta","webhook_url":"https://grok.example.invalid/hook/beta","token_file":"/run/secrets/beta/token"}]}`),
		"/run/secrets/beta/token": []byte("  file-token-value  \n"),
	}

	cfg, err := Load(LoadOptions{
		ConfigPath: "/etc/relay/config.json",
		ReadFile: func(path string) ([]byte, error) {
			data, ok := files[path]
			if !ok {
				return nil, errors.New("not found")
			}
			return data, nil
		},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bots[0].Token.Value() != "file-token-value" {
		t.Fatalf("token = %q", cfg.Bots[0].Token.Value())
	}
}

func TestLoadAddsBotWithoutCodeChange(t *testing.T) {
	t.Parallel()

	const twoBots = `{
	  "bots": [
	    {"id":"orimar","webhook_url":"https://grok.example.invalid/hook/orimar","token_env":"T_ORIMAR"},
	    {"id":"helper","webhook_url":"https://grok.example.invalid/hook/helper","token_env":"T_HELPER"}
	  ]
	}`

	env := map[string]string{"T_ORIMAR": "tok-a", "T_HELPER": "tok-b"}
	cfg, err := Load(LoadOptions{
		ConfigJSON: twoBots,
		LookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Bots) != 2 {
		t.Fatalf("bots len = %d, want 2", len(cfg.Bots))
	}
}

func TestLoadErrorsDoNotLeakSecrets(t *testing.T) {
	t.Parallel()

	sensitive := "leak-me-not-token-xyz"
	_, err := Load(LoadOptions{
		ConfigJSON: fmt.Sprintf(`{"bots":[{"id":"x","webhook_url":"https://grok.example.invalid/h/%s","token_env":"MISSING_ENV"}]}`, sensitive),
		LookupEnv:  func(string) (string, bool) { return "", false },
	})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, sensitive) {
		t.Fatalf("error leaked webhook path segment: %q", msg)
	}
}

func TestLoadFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts LoadOptions
		want string
	}{
		{
			name: "no source",
			opts: LoadOptions{},
			want: "set RELAY_CONFIG",
		},
		{
			name: "missing file",
			opts: LoadOptions{
				ConfigPath: "/missing/config.json",
				ReadFile:   func(string) ([]byte, error) { return nil, fs.ErrNotExist },
			},
			want: "file not found",
		},
		{
			name: "invalid json",
			opts: LoadOptions{ConfigJSON: `{`},
			want: "invalid JSON",
		},
		{
			name: "empty bots",
			opts: LoadOptions{ConfigJSON: `{"bots":[]}`},
			want: "at least one bot",
		},
		{
			name: "missing id",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"webhook_url":"https://grok.example.invalid/h","token_env":"T"}]}`,
				LookupEnv:  func(string) (string, bool) { return "tok", true },
			},
			want: "id is required",
		},
		{
			name: "missing webhook",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","token_env":"T"}]}`,
				LookupEnv:  func(string) (string, bool) { return "tok", true },
			},
			want: "webhook_url is required",
		},
		{
			name: "missing token source",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","webhook_url":"https://grok.example.invalid/h"}]}`,
			},
			want: "token_env or token_file is required",
		},
		{
			name: "both token sources",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","webhook_url":"https://grok.example.invalid/h","token_env":"T","token_file":"/x"}]}`,
			},
			want: "only one of token_env or token_file",
		},
		{
			name: "duplicate id",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"dup","webhook_url":"https://grok.example.invalid/h1","token_env":"T1"},{"id":"dup","webhook_url":"https://grok.example.invalid/h2","token_env":"T2"}]}`,
				LookupEnv: func(key string) (string, bool) {
					return "tok", key == "T1" || key == "T2"
				},
			},
			want: `duplicate bot id "dup"`,
		},
		{
			name: "empty token env",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","webhook_url":"https://grok.example.invalid/h","token_env":"EMPTY"}]}`,
				LookupEnv:  func(string) (string, bool) { return "", true },
			},
			want: "missing or empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(tt.opts)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestBotErrorMessagesDoNotLeakLoadedSecrets(t *testing.T) {
	t.Parallel()

	secret := "loaded-but-must-not-appear"
	_, err := Load(LoadOptions{
		ConfigJSON: `{"bots":[{"id":"","webhook_url":"https://grok.example.invalid/h","token_env":"T"}]}`,
		LookupEnv: func(string) (string, bool) {
			return secret, true
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked token: %q", err.Error())
	}
}
