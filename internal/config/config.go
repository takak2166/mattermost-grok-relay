package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Bot maps a Mattermost bot user identifier to its Grok webhook and token.
type Bot struct {
	ID         string
	WebhookURL WebhookURL
	Token      Secret
}

// Config holds relay configuration loaded from file or environment.
type Config struct {
	Bots []Bot
}

type rawConfig struct {
	Bots []rawBot `json:"bots"`
}

type rawBot struct {
	ID         string `json:"id"`
	WebhookURL string `json:"webhook_url"`
	TokenEnv   string `json:"token_env"`
	TokenFile  string `json:"token_file"`
}

// LoadOptions selects where configuration and secrets are read from.
type LoadOptions struct {
	ConfigPath string
	ConfigJSON string
	LookupEnv  func(string) (string, bool)
	ReadFile   func(string) ([]byte, error)
}

// Load reads and validates relay configuration. It fails closed on any error.
func Load(opts LoadOptions) (Config, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}

	raw, err := readRawConfig(opts)
	if err != nil {
		return Config{}, err
	}

	if len(raw.Bots) == 0 {
		return Config{}, fmt.Errorf("config: at least one bot entry is required")
	}

	seen := make(map[string]struct{}, len(raw.Bots))
	bots := make([]Bot, 0, len(raw.Bots))
	for i, entry := range raw.Bots {
		bot, err := resolveBot(entry, i, seen, opts)
		if err != nil {
			return Config{}, err
		}
		bots = append(bots, bot)
	}

	return Config{Bots: bots}, nil
}

func readRawConfig(opts LoadOptions) (rawConfig, error) {
	var payload []byte
	switch {
	case opts.ConfigJSON != "":
		payload = []byte(opts.ConfigJSON)
	case opts.ConfigPath != "":
		data, err := opts.ReadFile(opts.ConfigPath)
		if err != nil {
			if os.IsNotExist(err) {
				return rawConfig{}, fmt.Errorf("config: file not found")
			}
			return rawConfig{}, fmt.Errorf("config: read file: %w", err)
		}
		payload = data
	default:
		return rawConfig{}, fmt.Errorf("config: set RELAY_CONFIG, RELAY_CONFIG_JSON, or -config")
	}

	var raw rawConfig
	if err := json.Unmarshal(payload, &raw); err != nil {
		return rawConfig{}, fmt.Errorf("config: invalid JSON: %w", err)
	}
	return raw, nil
}

func resolveBot(entry rawBot, index int, seen map[string]struct{}, opts LoadOptions) (Bot, error) {
	prefix := fmt.Sprintf("config: bots[%d]", index)

	id := strings.TrimSpace(entry.ID)
	if id == "" {
		return Bot{}, fmt.Errorf("%s: id is required", prefix)
	}
	if _, dup := seen[id]; dup {
		return Bot{}, fmt.Errorf("config: duplicate bot id %q", id)
	}
	seen[id] = struct{}{}

	webhook := strings.TrimSpace(entry.WebhookURL)
	if webhook == "" {
		return Bot{}, fmt.Errorf("%s: webhook_url is required", prefix)
	}

	tokenEnv := strings.TrimSpace(entry.TokenEnv)
	tokenFile := strings.TrimSpace(entry.TokenFile)
	switch {
	case tokenEnv == "" && tokenFile == "":
		return Bot{}, fmt.Errorf("%s: token_env or token_file is required", prefix)
	case tokenEnv != "" && tokenFile != "":
		return Bot{}, fmt.Errorf("%s: specify only one of token_env or token_file", prefix)
	}

	token, err := loadToken(tokenEnv, tokenFile, opts)
	if err != nil {
		return Bot{}, fmt.Errorf("%s: %w", prefix, err)
	}
	if token == "" {
		return Bot{}, fmt.Errorf("%s: token is empty", prefix)
	}

	return Bot{
		ID:         id,
		WebhookURL: newWebhookURL(webhook),
		Token:      newSecret(token),
	}, nil
}

func loadToken(tokenEnv, tokenFile string, opts LoadOptions) (string, error) {
	if tokenEnv != "" {
		value, ok := opts.LookupEnv(tokenEnv)
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("environment variable %q is missing or empty", tokenEnv)
		}
		return strings.TrimSpace(value), nil
	}

	data, err := opts.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("token file not found")
		}
		return "", fmt.Errorf("read token file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ConfigPathFromEnv returns RELAY_CONFIG when set.
func ConfigPathFromEnv() (string, bool) {
	return lookupTrimmed(os.LookupEnv, "RELAY_CONFIG")
}

// ConfigJSONFromEnv returns RELAY_CONFIG_JSON when set.
func ConfigJSONFromEnv() (string, bool) {
	return lookupTrimmed(os.LookupEnv, "RELAY_CONFIG_JSON")
}

func lookupTrimmed(lookup func(string) (string, bool), key string) (string, bool) {
	value, ok := lookup(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}
