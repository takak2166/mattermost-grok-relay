package mattermost

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// BotIdentity holds the Mattermost user id and username for mention matching.
type BotIdentity struct {
	UserID   string
	Username string
}

// ResolveBotIdentity uses Client4 to load the bot user referenced in config.
func ResolveBotIdentity(ctx context.Context, siteURL, token, wantUserID string, httpClient *http.Client) (BotIdentity, error) {
	if err := ctx.Err(); err != nil {
		return BotIdentity{}, err
	}
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	client := model.NewAPIv4Client(siteURL)
	client.HTTPClient = httpClient
	client.SetToken(token)
	me, _, err := client.GetMe(ctx, "")
	if err != nil {
		return BotIdentity{}, fmt.Errorf("mattermost: get me: %w", err)
	}
	if me.Id != wantUserID {
		return BotIdentity{}, fmt.Errorf("mattermost: token user does not match configured bot id")
	}
	return BotIdentity{UserID: me.Id, Username: me.Username}, nil
}
