package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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
	ID        string `json:"id"`
	URLEnv    string `json:"url_env"`
	URLFile   string `json:"url_file"`
	TokenEnv  string `json:"token_env"`
	TokenFile string `json:"token_file"`
}

// LoadOptions selects where configuration and secrets are read from.
// When both ConfigPath and ConfigJSON are set, Load returns an error.
type LoadOptions struct {
	ConfigPath string
	ConfigJSON string
	LookupEnv  func(string) (string, bool)
	ReadFile   func(string) ([]byte, error)
}

func (opts *LoadOptions) ensureDefaults() {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
}

// ApplySources fills ConfigPath or ConfigJSON from the -config flag and environment.
// Precedence: flagConfigPath, then RELAY_CONFIG, then RELAY_CONFIG_JSON.
func (opts *LoadOptions) ApplySources(flagConfigPath string) {
	opts.ensureDefaults()
	if path := strings.TrimSpace(flagConfigPath); path != "" {
		opts.ConfigPath = path
		return
	}
	if path, ok := lookupTrimmed(opts.LookupEnv, "RELAY_CONFIG"); ok {
		opts.ConfigPath = path
		return
	}
	if inline, ok := lookupTrimmed(opts.LookupEnv, "RELAY_CONFIG_JSON"); ok {
		opts.ConfigJSON = inline
	}
}

// Load reads and validates relay configuration. It fails closed on any error.
func Load(opts LoadOptions) (Config, error) {
	opts.ensureDefaults()

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
	if opts.ConfigJSON != "" && opts.ConfigPath != "" {
		return rawConfig{}, fmt.Errorf("config: config file path and inline JSON are mutually exclusive")
	}

	var payload []byte
	switch {
	case opts.ConfigJSON != "":
		payload = []byte(opts.ConfigJSON)
	case opts.ConfigPath != "":
		data, err := opts.ReadFile(opts.ConfigPath)
		if err != nil {
			return rawConfig{}, fmt.Errorf("config: read %q: %w", opts.ConfigPath, err)
		}
		payload = data
	default:
		return rawConfig{}, fmt.Errorf("config: set RELAY_CONFIG, RELAY_CONFIG_JSON, or -config")
	}

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()

	var raw rawConfig
	if err := dec.Decode(&raw); err != nil {
		return rawConfig{}, fmt.Errorf("config: invalid JSON: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return rawConfig{}, fmt.Errorf("config: invalid JSON: trailing data after config object")
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

	urlEnv := strings.TrimSpace(entry.URLEnv)
	urlFile := strings.TrimSpace(entry.URLFile)
	switch {
	case urlEnv == "" && urlFile == "":
		return Bot{}, fmt.Errorf("%s: url_env or url_file is required", prefix)
	case urlEnv != "" && urlFile != "":
		return Bot{}, fmt.Errorf("%s: specify only one of url_env or url_file", prefix)
	}

	webhook, err := loadEnvOrFile(urlEnv, urlFile, opts, "url")
	if err != nil {
		return Bot{}, fmt.Errorf("%s: %w", prefix, err)
	}
	if webhook == "" {
		return Bot{}, fmt.Errorf("%s: url value is empty", prefix)
	}
	if err := validateWebhookURL(webhook); err != nil {
		return Bot{}, fmt.Errorf("%s (id %q): %w", prefix, id, err)
	}

	tokenEnv := strings.TrimSpace(entry.TokenEnv)
	tokenFile := strings.TrimSpace(entry.TokenFile)
	switch {
	case tokenEnv == "" && tokenFile == "":
		return Bot{}, fmt.Errorf("%s: token_env or token_file is required", prefix)
	case tokenEnv != "" && tokenFile != "":
		return Bot{}, fmt.Errorf("%s: specify only one of token_env or token_file", prefix)
	}

	token, err := loadEnvOrFile(tokenEnv, tokenFile, opts, "token")
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

func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("url value must be a valid https URL with a hostname")
	}
	return nil
}

func loadEnvOrFile(envName, filePath string, opts LoadOptions, kind string) (string, error) {
	if envName != "" {
		value, ok := opts.LookupEnv(envName)
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("environment variable %q is missing or empty", envName)
		}
		return strings.TrimSpace(value), nil
	}

	data, err := opts.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s file %q: %w", kind, filePath, err)
	}
	return strings.TrimSpace(string(data)), nil
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
