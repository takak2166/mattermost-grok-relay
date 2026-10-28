package config

import (
	_ "embed"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

//go:embed testdata/example.json
var exampleConfigJSON []byte

func testLookup(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func defaultBotEnv() map[string]string {
	return map[string]string{
		"MM_URL_ORIMAR":   testWebhook,
		"MM_TOKEN_ORIMAR": testToken,
	}
}

func TestLoadExampleFixture(t *testing.T) {
	t.Parallel()

	cfg, err := Load(LoadOptions{
		ConfigJSON: string(exampleConfigJSON),
		LookupEnv: testLookup(map[string]string{
			"RELAY_BOT_WEBHOOK_URL_ORIMAR": testWebhook,
			"RELAY_BOT_TOKEN_ORIMAR":       testToken,
		}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bots[0].ID != "orimar-bot-user-id" {
		t.Fatalf("id = %q", cfg.Bots[0].ID)
	}
}

func TestApplySourcesPrecedence(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"RELAY_CONFIG":      "/env/config.json",
		"RELAY_CONFIG_JSON": `{"bots":[]}`,
	}
	lookup := testLookup(env)

	opts := LoadOptions{LookupEnv: lookup}
	opts.ApplySources("/flag/config.json")
	if opts.ConfigPath != "/flag/config.json" || opts.ConfigJSON != "" {
		t.Fatalf("flag path: %+v", opts)
	}

	opts = LoadOptions{LookupEnv: lookup}
	opts.ApplySources("")
	if opts.ConfigPath != "/env/config.json" || opts.ConfigJSON != "" {
		t.Fatalf("RELAY_CONFIG: %+v", opts)
	}

	delete(env, "RELAY_CONFIG")
	opts = LoadOptions{LookupEnv: lookup}
	opts.ApplySources("")
	if opts.ConfigJSON == "" || opts.ConfigPath != "" {
		t.Fatalf("RELAY_CONFIG_JSON: %+v", opts)
	}
}

func TestLoadSuccessFromJSON(t *testing.T) {
	t.Parallel()

	cfg, err := Load(LoadOptions{
		ConfigJSON: `{"bots":[{"id":"orimar","url_env":"MM_URL_ORIMAR","token_env":"MM_TOKEN_ORIMAR"}]}`,
		LookupEnv:  testLookup(defaultBotEnv()),
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
	if cfg.Bots[0].WebhookURL.Value() != testWebhook {
		t.Fatalf("webhook mismatch")
	}
}

func TestLoadSuccessFromURLAndTokenFiles(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"/etc/relay/config.json":  []byte(`{"bots":[{"id":"beta","url_file":"/run/secrets/beta/url","token_file":"/run/secrets/beta/token"}]}`),
		"/run/secrets/beta/url":   []byte("  https://grok.example.invalid/hook/beta  \n"),
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
	if cfg.Bots[0].WebhookURL.Value() != "https://grok.example.invalid/hook/beta" {
		t.Fatalf("webhook = %q", cfg.Bots[0].WebhookURL.Value())
	}
}

func TestLoadAddsBotWithoutCodeChange(t *testing.T) {
	t.Parallel()

	const twoBots = `{
	  "bots": [
	    {"id":"orimar","url_env":"U_ORIMAR","token_env":"T_ORIMAR"},
	    {"id":"helper","url_env":"U_HELPER","token_env":"T_HELPER"}
	  ]
	}`

	env := map[string]string{
		"U_ORIMAR": "https://grok.example.invalid/hook/orimar",
		"T_ORIMAR": "tok-a",
		"U_HELPER": "https://grok.example.invalid/hook/helper",
		"T_HELPER": "tok-b",
	}
	cfg, err := Load(LoadOptions{
		ConfigJSON: twoBots,
		LookupEnv:  testLookup(env),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Bots) != 2 {
		t.Fatalf("bots len = %d, want 2", len(cfg.Bots))
	}
}

func TestLoadRejectsLegacyWebhookURLField(t *testing.T) {
	t.Parallel()

	_, err := Load(LoadOptions{
		ConfigJSON: `{"bots":[{"id":"a","webhook_url":"https://grok.example.invalid/h","token_env":"T"}]}`,
		LookupEnv:  testLookup(map[string]string{"T": testToken}),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadErrorsDoNotLeakWebhookURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid scheme",
			url:  "http://grok.example.invalid/secret-path?key=must-not-leak",
		},
		{
			name: "url parse error",
			url:  "https://grok.example.invalid/se cret-path?key=must-not-leak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(LoadOptions{
				ConfigJSON: `{"bots":[{"id":"x","url_env":"SECRET_URL","token_env":"T"}]}`,
				LookupEnv: testLookup(map[string]string{
					"SECRET_URL": tt.url,
					"T":          testToken,
				}),
			})
			if err == nil {
				t.Fatal("expected error")
			}
			msg := err.Error()
			if strings.Contains(msg, tt.url) {
				t.Fatalf("error leaked webhook URL: %q", msg)
			}
			if !strings.Contains(msg, `id "x"`) {
				t.Fatalf("error should mention bot id: %q", msg)
			}
		})
	}
}

func TestLoadFailClosed(t *testing.T) {
	t.Parallel()

	baseEnv := defaultBotEnv()

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
			want: "read \"/missing/config.json\"",
		},
		{
			name: "mutually exclusive sources",
			opts: LoadOptions{
				ConfigPath: "/a.json",
				ConfigJSON: `{"bots":[]}`,
			},
			want: "mutually exclusive",
		},
		{
			name: "unknown json field",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_envv":"T"}]}`,
			},
			want: "invalid JSON",
		},
		{
			name: "legacy webhook_url field",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","webhook_url":"https://grok.example.invalid/h","token_env":"T"}]}`,
			},
			want: "invalid JSON",
		},
		{
			name: "invalid webhook scheme",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_env":"T"}]}`,
				LookupEnv: testLookup(map[string]string{
					"U": "http://grok.example.invalid/h",
					"T": testToken,
				}),
			},
			want: "valid https URL",
		},
		{
			name: "invalid webhook value",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_env":"T"}]}`,
				LookupEnv: testLookup(map[string]string{
					"U": "not-a-url",
					"T": testToken,
				}),
			},
			want: "valid https URL",
		},
		{
			name: "webhook missing hostname",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_env":"T"}]}`,
				LookupEnv: testLookup(map[string]string{
					"U": "https://:443/hook",
					"T": testToken,
				}),
			},
			want: "valid https URL",
		},
		{
			name: "trailing json after config",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_env":"T"}]} {"bots":[]}`,
				LookupEnv:  testLookup(baseEnv),
			},
			want: "trailing data",
		},
		{
			name: "second bot validation index",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"ok","url_env":"U1","token_env":"T1"},{"id":"","url_env":"U2","token_env":"T2"}]}`,
				LookupEnv: testLookup(map[string]string{
					"U1": testWebhook, "T1": testToken,
					"U2": testWebhook, "T2": testToken,
				}),
			},
			want: "bots[1]",
		},
		{
			name: "url file missing",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_file":"/missing/url","token_env":"T"}]}`,
				LookupEnv:  testLookup(map[string]string{"T": testToken}),
				ReadFile: func(path string) ([]byte, error) {
					return nil, fs.ErrNotExist
				},
			},
			want: "read url file",
		},
		{
			name: "token file missing",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_file":"/missing/token"}]}`,
				LookupEnv:  testLookup(map[string]string{"U": testWebhook}),
				ReadFile: func(path string) ([]byte, error) {
					return nil, fs.ErrNotExist
				},
			},
			want: "read token file",
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
				ConfigJSON: `{"bots":[{"url_env":"U","token_env":"T"}]}`,
				LookupEnv:  testLookup(baseEnv),
			},
			want: "id is required",
		},
		{
			name: "missing url source",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","token_env":"T"}]}`,
				LookupEnv:  testLookup(map[string]string{"T": testToken}),
			},
			want: "url_env or url_file is required",
		},
		{
			name: "both url sources",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","url_file":"/x","token_env":"T"}]}`,
			},
			want: "only one of url_env or url_file",
		},
		{
			name: "empty url env",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"EMPTY_URL","token_env":"T"}]}`,
				LookupEnv: testLookup(map[string]string{
					"EMPTY_URL": "",
					"T":         testToken,
				}),
			},
			want: "missing or empty",
		},
		{
			name: "missing token source",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U"}]}`,
				LookupEnv:  testLookup(map[string]string{"U": testWebhook}),
			},
			want: "token_env or token_file is required",
		},
		{
			name: "both token sources",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_env":"T","token_file":"/x"}]}`,
				LookupEnv:  testLookup(map[string]string{"U": testWebhook}),
			},
			want: "only one of token_env or token_file",
		},
		{
			name: "duplicate id",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"dup","url_env":"U1","token_env":"T1"},{"id":"dup","url_env":"U2","token_env":"T2"}]}`,
				LookupEnv: testLookup(map[string]string{
					"U1": testWebhook, "T1": testToken,
					"U2": testWebhook, "T2": testToken,
				}),
			},
			want: `duplicate bot id "dup"`,
		},
		{
			name: "empty token env",
			opts: LoadOptions{
				ConfigJSON: `{"bots":[{"id":"a","url_env":"U","token_env":"EMPTY"}]}`,
				LookupEnv: testLookup(map[string]string{
					"U": testWebhook,
				}),
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
		ConfigJSON: `{"bots":[{"id":"","url_env":"U","token_env":"T"}]}`,
		LookupEnv: testLookup(map[string]string{
			"U": testWebhook,
			"T": secret,
		}),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked token: %q", err.Error())
	}
	if strings.Contains(err.Error(), testWebhook) {
		t.Fatalf("error leaked webhook URL: %q", err.Error())
	}
}
