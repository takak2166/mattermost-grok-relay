package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

const redacted = "[REDACTED]"

// Secret holds a sensitive token value that must not appear in logs or errors.
type Secret struct {
	value string
}

func newSecret(value string) Secret {
	return Secret{value: value}
}

// String returns a redacted representation.
func (s Secret) String() string {
	return redacted
}

// GoString returns a redacted representation for %#v formatting.
func (s Secret) GoString() string {
	return redacted
}

// LogValue implements slog.LogValuer so structured logs never emit the raw token.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON emits a redacted JSON value.
func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(redacted)
}

// UnmarshalJSON rejects direct JSON decoding of secrets (tokens come from env or files).
func (s *Secret) UnmarshalJSON([]byte) error {
	return fmt.Errorf("secret values cannot be loaded from config JSON; use token_env or token_file")
}

// Value returns the raw secret for use by trusted code only.
func (s Secret) Value() string {
	return s.value
}

// WebhookURL holds a Grok Bot inbound webhook URL treated as a secret.
type WebhookURL struct {
	value string
}

func newWebhookURL(value string) WebhookURL {
	return WebhookURL{value: value}
}

// String returns a redacted representation.
func (u WebhookURL) String() string {
	return redacted
}

// GoString returns a redacted representation for %#v formatting.
func (u WebhookURL) GoString() string {
	return redacted
}

// LogValue implements slog.LogValuer.
func (u WebhookURL) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON emits a redacted JSON value.
func (u WebhookURL) MarshalJSON() ([]byte, error) {
	return json.Marshal(redacted)
}

// UnmarshalJSON rejects direct JSON decoding of webhook URLs (use url_env or url_file).
func (u *WebhookURL) UnmarshalJSON([]byte) error {
	return fmt.Errorf("webhook URL values cannot be loaded from config JSON; use url_env or url_file")
}

// Value returns the raw URL for use by trusted code only.
func (u WebhookURL) Value() string {
	return u.value
}
