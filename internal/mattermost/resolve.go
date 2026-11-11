package mattermost

import (
	"context"
	"net/http"
	"time"
)

// DefaultHTTPTimeout bounds REST identity resolution.
const DefaultHTTPTimeout = 30 * time.Second

var defaultHTTPClient = &http.Client{Timeout: DefaultHTTPTimeout}

// ResolveIdentity loads the bot user for config validation and mention matching.
type ResolveIdentity func(ctx context.Context, siteURL, token, userID string) (BotIdentity, error)

func DefaultResolveIdentity(ctx context.Context, siteURL, token, userID string) (BotIdentity, error) {
	return ResolveBotIdentity(ctx, siteURL, token, userID, defaultHTTPClient)
}
