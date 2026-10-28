package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const testToken = "mm-bot-token-super-secret-value"
const testWebhook = "https://grok.example.invalid/routines/orimar/inbound?key=abc123"

func TestSecretDoesNotLeak(t *testing.T) {
	t.Parallel()
	s := newSecret(testToken)

	assertNoLeak(t, testToken, s.String())
	assertNoLeak(t, testToken, s.GoString())
	assertNoLeak(t, testToken, fmt.Sprintf("%v", s))
	assertNoLeak(t, testToken, fmt.Sprintf("%#v", s))

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("token", "secret", s)
	assertNoLeak(t, testToken, buf.String())

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	assertNoLeak(t, testToken, string(data))
}

func TestWebhookURLDoesNotLeak(t *testing.T) {
	t.Parallel()
	u := newWebhookURL(testWebhook)

	assertNoLeak(t, testWebhook, u.String())
	assertNoLeak(t, testWebhook, u.GoString())
	assertNoLeak(t, testWebhook, fmt.Sprintf("%v", u))
	assertNoLeak(t, testWebhook, fmt.Sprintf("%#v", u))

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("webhook", "url", u)
	assertNoLeak(t, testWebhook, buf.String())

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	assertNoLeak(t, testWebhook, string(data))
}

func TestSecretUnmarshalJSONRejected(t *testing.T) {
	t.Parallel()
	var s Secret
	if err := json.Unmarshal([]byte(`"raw-token"`), &s); err == nil {
		t.Fatal("expected error unmarshaling secret from JSON")
	}
}

func TestWebhookURLUnmarshalJSONRejected(t *testing.T) {
	t.Parallel()
	var u WebhookURL
	if err := json.Unmarshal([]byte(`"https://grok.example.invalid/h"`), &u); err == nil {
		t.Fatal("expected error unmarshaling webhook URL from JSON")
	}
}

func TestBotJSONRedactsSecrets(t *testing.T) {
	t.Parallel()
	b := Bot{
		ID:         "bot-a",
		WebhookURL: newWebhookURL(testWebhook),
		Token:      newSecret(testToken),
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := string(data)
	assertNoLeak(t, testToken, out)
	assertNoLeak(t, testWebhook, out)
}

func assertNoLeak(t *testing.T, secret, output string) {
	t.Helper()
	if strings.Contains(output, secret) {
		t.Fatalf("output leaked secret %q: %q", secret, output)
	}
	if secret != "" && output != "" && !strings.Contains(output, redacted) {
		t.Fatalf("output should contain %q: %q", redacted, output)
	}
}
