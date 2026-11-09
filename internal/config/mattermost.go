package config

import (
	"fmt"

	"github.com/takak2166/mattermost-grok-relay/internal/siteurl"
)

// MattermostServerURL returns the Mattermost site URL from RELAY_MATTERMOST_URL.
func MattermostServerURL(lookup func(string) (string, bool)) (string, error) {
	raw, ok := lookupTrimmed(lookup, "RELAY_MATTERMOST_URL")
	if !ok {
		return "", fmt.Errorf("config: RELAY_MATTERMOST_URL is required")
	}
	normalized, err := siteurl.Normalize(raw)
	if err != nil {
		return "", fmt.Errorf("config: RELAY_MATTERMOST_URL %w", err)
	}
	return normalized, nil
}
