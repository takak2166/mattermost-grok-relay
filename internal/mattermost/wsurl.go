package mattermost

import (
	"fmt"
	"net/url"
)

// WebSocketOrigin converts an http(s) Mattermost site URL to a ws(s) origin
// suitable for model.NewWebSocketClient4 (scheme + host only).
func WebSocketOrigin(siteURL string) (string, error) {
	u, err := url.Parse(siteURL)
	if err != nil {
		return "", fmt.Errorf("mattermost: invalid site URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("mattermost: site URL scheme must be http or https")
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return u.String(), nil
}
