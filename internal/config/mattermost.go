package config

import (
	"fmt"
	"net/url"
	"strings"
)

// MattermostServerURL returns the Mattermost site URL from RELAY_MATTERMOST_URL.
func MattermostServerURL(lookup func(string) (string, bool)) (string, error) {
	raw, ok := lookupTrimmed(lookup, "RELAY_MATTERMOST_URL")
	if !ok {
		return "", fmt.Errorf("config: RELAY_MATTERMOST_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("config: RELAY_MATTERMOST_URL must be a valid http(s) URL with a hostname")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("config: RELAY_MATTERMOST_URL scheme must be http or https")
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
