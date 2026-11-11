package siteurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Normalize trims a Mattermost site URL for REST and WebSocket clients.
// Subpath is preserved; query, fragment, and userinfo are removed.
func Normalize(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("site URL must be a valid http(s) URL with a hostname")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("site URL scheme must be http or https")
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
