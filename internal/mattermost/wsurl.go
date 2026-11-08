package mattermost

// Deprecated: use WebSocketSiteURL. Kept as a thin alias for callers in this package.
func WebSocketOrigin(siteURL string) (string, error) {
	return WebSocketSiteURL(siteURL)
}
