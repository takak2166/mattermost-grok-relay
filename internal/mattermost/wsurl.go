package mattermost

// Deprecated: use WebSocketSiteURL.
func WebSocketOrigin(siteURL string) (string, error) {
	return WebSocketSiteURL(siteURL)
}
