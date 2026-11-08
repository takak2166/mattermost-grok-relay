package mattermost

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

// BotIdentity holds the Mattermost user id and username for mention matching.
type BotIdentity struct {
	UserID   string
	Username string
}

// ResolveBotIdentity uses Client4 to load the bot user referenced in config.
func ResolveBotIdentity(siteURL, token, wantUserID string) (BotIdentity, error) {
	client := model.NewAPIv4Client(siteURL)
	client.SetToken(token)
	user, _, err := client.GetUser(context.Background(), wantUserID, "")
	if err != nil {
		return BotIdentity{}, fmt.Errorf("mattermost: get user %q: %w", wantUserID, err)
	}
	if user.Id != wantUserID {
		return BotIdentity{}, fmt.Errorf("mattermost: user id mismatch")
	}
	return BotIdentity{UserID: user.Id, Username: user.Username}, nil
}
