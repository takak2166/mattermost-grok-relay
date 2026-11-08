package mattermost

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeSiteURL trims a Mattermost site URL for Client4 / WebSocketClient4.
// The URL path (subpath install) is preserved; only query, fragment, and userinfo are removed.
func NormalizeSiteURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("mattermost: site URL must be a valid http(s) URL with a hostname")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("mattermost: site URL scheme must be http or https")
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	if u.Path != "" && u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	u.RawPath = ""
	return strings.TrimRight(u.String(), "/"), nil
}

// WebSocketSiteURL converts an http(s) site URL to ws(s) for NewWebSocketClient4.
func WebSocketSiteURL(siteURL string) (string, error) {
	normalized, err := NormalizeSiteURL(siteURL)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(normalized)
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
	return u.String(), nil
}
