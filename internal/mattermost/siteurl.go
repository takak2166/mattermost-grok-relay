package mattermost

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/takak2166/mattermost-grok-relay/internal/siteurl"
)

// NormalizeSiteURL trims a Mattermost site URL for Client4 / WebSocketClient4.
func NormalizeSiteURL(raw string) (string, error) {
	normalized, err := siteurl.Normalize(raw)
	if err != nil {
		return "", fmt.Errorf("mattermost: %w", err)
	}
	return normalized, nil
}

// WebSocketSiteURL converts an http(s) site URL to ws(s) for NewWebSocketClient4.
func WebSocketSiteURL(siteURL string) (string, error) {
	normalized, err := NormalizeSiteURL(siteURL)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("mattermost: site URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("mattermost: site URL scheme must be http or https")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
